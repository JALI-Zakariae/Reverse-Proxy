package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run backend.go <port>")
	}
	
	port := os.Args[1]
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[%s] Received request: %s %s", port, r.Method, r.URL.Path)
		fmt.Fprintf(w, "Response from backend on port %s at %s\n", port, time.Now().Format(time.RFC3339))
	})
	
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})
	
	http.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		fmt.Fprintf(w, "Slow response from backend on port %s\n", port)
	})
	
	addr := ":" + port
	log.Printf("Starting backend server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}