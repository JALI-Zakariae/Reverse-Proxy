package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type Backend struct {
	URL          *url.URL `json:"url"`
	Alive        bool     `json:"alive"`
	CurrentConns int64    `json:"current_connections"`
	mux          sync.RWMutex
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	defer b.mux.Unlock()
	b.Alive = alive
}

func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Alive
}

func (b *Backend) IncrementConnections() {
	atomic.AddInt64(&b.CurrentConns, 1)
}

func (b *Backend) DecrementConnections() {
	atomic.AddInt64(&b.CurrentConns, -1)
}

func (b *Backend) GetConnections() int64 {
	return atomic.LoadInt64(&b.CurrentConns)
}

type ServerPool struct {
	Backends []*Backend
	Current  uint64
	mux      sync.RWMutex
}

type LoadBalancer interface {
	GetNextValidPeer() *Backend
	AddBackend(backend *Backend)
	SetBackendStatus(uri *url.URL, alive bool)
}

func (s *ServerPool) AddBackend(backend *Backend) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.Backends = append(s.Backends, backend)
}

func (s *ServerPool) RemoveBackend(urlStr string) bool {
	s.mux.Lock()
	defer s.mux.Unlock()
	
	for i, backend := range s.Backends {
		if backend.URL.String() == urlStr {
			s.Backends = append(s.Backends[:i], s.Backends[i+1:]...)
			return true
		}
	}
	return false
}

func (s *ServerPool) SetBackendStatus(uri *url.URL, alive bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	for _, backend := range s.Backends {
		if backend.URL.String() == uri.String() {
			backend.SetAlive(alive)
			if !alive {
				log.Printf("Backend %s is DOWN", uri.String())
			} else {
				log.Printf("Backend %s is UP", uri.String())
			}
			break
		}
	}
}

func (s *ServerPool) GetNextValidPeer() *Backend {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	if len(s.Backends) == 0 {
		return nil
	}
	
	next := atomic.AddUint64(&s.Current, 1)
	l := uint64(len(s.Backends))
	
	for i := uint64(0); i < l; i++ {
		idx := (next + i) % l
		backend := s.Backends[idx]
		if backend.IsAlive() {
			return backend
		}
	}
	
	return nil
}

func (s *ServerPool) GetLeastConnectedPeer() *Backend {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	var selected *Backend
	minConns := int64(-1)
	
	for _, backend := range s.Backends {
		if !backend.IsAlive() {
			continue
		}
		
		conns := backend.GetConnections()
		if minConns == -1 || conns < minConns {
			minConns = conns
			selected = backend
		}
	}
	
	return selected
}

func (s *ServerPool) GetAllBackends() []map[string]interface{} {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	result := make([]map[string]interface{}, len(s.Backends))
	for i, backend := range s.Backends {
		result[i] = map[string]interface{}{
			"url":                 backend.URL.String(),
			"alive":               backend.IsAlive(),
			"current_connections": backend.GetConnections(),
		}
	}
	return result
}

type ProxyConfig struct {
	Port            int           `json:"port"`
	AdminPort       int           `json:"admin_port"`
	Strategy        string        `json:"strategy"`
	HealthCheckFreq time.Duration `json:"health_check_frequency"`
	Backends        []string      `json:"backends"`
}

type ProxyServer struct {
	config     *ProxyConfig
	serverPool *ServerPool
}

func NewProxyServer(config *ProxyConfig) *ProxyServer {
	return &ProxyServer{
		config:     config,
		serverPool: &ServerPool{},
	}
}

func (p *ProxyServer) loadBackends() error {
	for _, backendURL := range p.config.Backends {
		uri, err := url.Parse(backendURL)
		if err != nil {
			return fmt.Errorf("failed to parse backend URL %s: %v", backendURL, err)
		}
		
		backend := &Backend{
			URL:   uri,
			Alive: true,
		}
		p.serverPool.AddBackend(backend)
		log.Printf("Added backend: %s", backendURL)
	}
	return nil
}

func isBackendAlive(ctx context.Context, uri *url.URL) bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", uri.String()+"/health", nil)
	if err != nil {
		req, err = http.NewRequestWithContext(ctx, "HEAD", uri.String(), nil)
		if err != nil {
			return false
		}
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode < 500
}

func (p *ProxyServer) healthCheck(ctx context.Context) {
	ticker := time.NewTicker(p.config.HealthCheckFreq)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Health checker stopped")
			return
		case <-ticker.C:
			p.serverPool.mux.RLock()
			backends := make([]*Backend, len(p.serverPool.Backends))
			copy(backends, p.serverPool.Backends)
			p.serverPool.mux.RUnlock()
			
			for _, backend := range backends {
				alive := isBackendAlive(ctx, backend.URL)
				backend.SetAlive(alive)
				
				status := "UP"
				if !alive {
					status = "DOWN"
				}
				log.Printf("Health check: %s is %s", backend.URL.String(), status)
			}
		}
	}
}

func (p *ProxyServer) proxyHandler(w http.ResponseWriter, r *http.Request) {
	var backend *Backend
	
	if p.config.Strategy == "least-conn" {
		backend = p.serverPool.GetLeastConnectedPeer()
	} else {
		backend = p.serverPool.GetNextValidPeer()
	}
	
	if backend == nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	
	backend.IncrementConnections()
	defer backend.DecrementConnections()
	
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("Error proxying to %s: %v", backend.URL.String(), err)
		p.serverPool.SetBackendStatus(backend.URL, false)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
	
	proxy.ServeHTTP(w, r)
}

func (p *ProxyServer) adminStatusHandler(w http.ResponseWriter, r *http.Request) {
	backends := p.serverPool.GetAllBackends()
	
	activeCount := 0
	for _, b := range backends {
		if b["alive"].(bool) {
			activeCount++
		}
	}
	
	response := map[string]interface{}{
		"total_backends":  len(backends),
		"active_backends": activeCount,
		"backends":        backends,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (p *ProxyServer) adminAddBackendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		URL string `json:"url"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	uri, err := url.Parse(req.URL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	
	backend := &Backend{
		URL:   uri,
		Alive: true,
	}
	
	p.serverPool.AddBackend(backend)
	log.Printf("Added backend via API: %s", req.URL)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Backend added successfully",
		"url":     req.URL,
	})
}

func (p *ProxyServer) adminRemoveBackendHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		URL string `json:"url"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if p.serverPool.RemoveBackend(req.URL) {
		log.Printf("Removed backend via API: %s", req.URL)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Backend removed successfully",
			"url":     req.URL,
		})
	} else {
		http.Error(w, "Backend not found", http.StatusNotFound)
	}
}

func (p *ProxyServer) Start(ctx context.Context) error {
	if err := p.loadBackends(); err != nil {
		return err
	}
	
	go p.healthCheck(ctx)
	
	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", p.proxyHandler)
	
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/status", p.adminStatusHandler)
	adminMux.HandleFunc("/backends", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			p.adminAddBackendHandler(w, r)
		case http.MethodDelete:
			p.adminRemoveBackendHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	proxyServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", p.config.Port),
		Handler: proxyMux,
	}
	
	adminServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", p.config.AdminPort),
		Handler: adminMux,
	}
	
	go func() {
		log.Printf("Starting proxy server on port %d", p.config.Port)
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Proxy server error: %v", err)
		}
	}()
	
	go func() {
		log.Printf("Starting admin server on port %d", p.config.AdminPort)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Admin server error: %v", err)
		}
	}()
	
	<-ctx.Done()
	
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	log.Println("Shutting down servers...")
	proxyServer.Shutdown(shutdownCtx)
	adminServer.Shutdown(shutdownCtx)
	
	return nil
}

func loadConfig(configPath string) (*ProxyConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var config ProxyConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}
	
	return &config, nil
}

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	proxy := NewProxyServer(config)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		cancel()
	}()
	
	if err := proxy.Start(ctx); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}