# Reverse-Proxy


A lightweight, configurable HTTP reverse proxy and load balancer written in Go. Supports multiple load balancing strategies, health checking, and a live admin API for dynamic backend management.

---

## Features

- **Round-robin** and **least-connections** load balancing strategies
- Automatic **health checks** with configurable frequency
- **Admin API** for real-time backend management (add, remove, inspect)
- Graceful shutdown on `SIGINT` / `SIGTERM`
- Concurrent-safe with atomic connection tracking and `sync.RWMutex`

---

## Project Structure

```
.
├── main.go                  # Proxy server, backend pool, admin API
├── backend.go               # Minimal backend server for testing
├── config.json              # Default config (round-robin)
├── config-least-conn.json   # Alternate config (least-connections)
├── go.mod
├── start-backend.ps1        # PowerShell: start test backends
├── stop-all.ps1             # PowerShell: stop all processes
└── test.ps1                 # PowerShell: run test requests
```

---

## Getting Started



### Run the Load Balancer

```bash
go run main.go -config config.json
```

To use least-connections strategy:

```bash
go run main.go -config config-least-conn.json
```

### Start Test Backends (PowerShell)

```powershell
.\start-backend.ps1
```

This starts three backend servers on ports `9001`, `9002`, and `9003`.

---

## Configuration

Configuration is defined in a JSON file:

```json
{
  "port": 8080,
  "admin_port": 8081,
  "strategy": "round-robin",
  "health_check_frequency": 30000000000,
  "backends": [
    "http://localhost:9001",
    "http://localhost:9002",
    "http://localhost:9003"
  ]
}
```

| Field | Description |
|---|---|
| `port` | Port the proxy listens on |
| `admin_port` | Port the admin API listens on |
| `strategy` | `round-robin` or `least-conn` |
| `health_check_frequency` | Health check interval in nanoseconds (e.g. `30000000000` = 30s) |
| `backends` | List of backend URLs |

---

## Load Balancing Strategies

### Round-Robin (`round-robin`)
Distributes requests evenly across all healthy backends in order. Good for homogeneous workloads.

### Least Connections (`least-conn`)
Routes each request to the backend with the fewest active connections. Better for workloads with variable response times.

---

## Admin API

The admin server runs on `admin_port` (default `8081`).

### Get Status

```http
GET http://localhost:8081/status
```

**Response:**
```json
{
  "total_backends": 3,
  "active_backends": 3,
  "backends": [
    {
      "url": "http://localhost:9001",
      "alive": true,
      "current_connections": 0
    }
  ]
}
```

### Add a Backend

```http
POST http://localhost:8081/backends
Content-Type: application/json

{ "url": "http://localhost:9004" }
```

### Remove a Backend

```http
DELETE http://localhost:8081/backends
Content-Type: application/json

{ "url": "http://localhost:9004" }
```

---

## Backend Server (for Testing)

`backend.go` is a simple HTTP server that can be run on any port to act as a test upstream.

```bash
go run backend.go 9001
```

It exposes three routes:

| Route | Description |
|---|---|
| `/` | Returns a response with the port and current timestamp |
| `/health` | Returns `200 OK` (used by health checker) |
| `/slow` | Sleeps for 3 seconds before responding (useful for testing least-conn) |

---

## Health Checking

Health checks run in the background at the configured frequency. Each backend is checked by sending a `GET` request to its `/health` endpoint. If the backend returns a `5xx` status or is unreachable, it is marked as **DOWN** and excluded from routing until it recovers.

Backends that fail during a proxied request are also immediately marked as **DOWN**.

---

## Graceful Shutdown

The proxy handles `SIGINT` (Ctrl+C) and `SIGTERM` signals. On shutdown, it allows up to 10 seconds for in-flight requests to complete before stopping both the proxy and admin servers.
