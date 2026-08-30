package backend

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

// SimulatedBackend simulates a downstream backend server with dynamic health toggling.
type SimulatedBackend struct {
	Addr    string
	healthy bool
	mu      sync.RWMutex
	server  *http.Server
}

// NewSimulatedBackend creates a new backend instance.
func NewSimulatedBackend(addr string) *SimulatedBackend {
	return &SimulatedBackend{
		Addr:    addr,
		healthy: true,
	}
}

// Start runs the HTTP server in a blocking call. It should be executed in a goroutine.
func (sb *SimulatedBackend) Start() error {
	mux := http.NewServeMux()

	// Main request routing
	mux.HandleFunc("/", sb.handleRequest)
	// Health endpoint queried by the Gateway Health Checker
	mux.HandleFunc("/health", sb.handleHealth)
	// Endpoint to manually simulate failure/recovery for interview demos
	mux.HandleFunc("/toggle", sb.handleToggle)

	sb.server = &http.Server{
		Addr:    sb.Addr,
		Handler: mux,
	}

	log.Printf("[Backend %s] Starting simulated backend...", sb.Addr)
	return sb.server.ListenAndServe()
}

// Close stops the HTTP server.
func (sb *SimulatedBackend) Close() error {
	if sb.server != nil {
		return sb.server.Close()
	}
	return nil
}

// handleRequest processes incoming proxied HTTP traffic.
func (sb *SimulatedBackend) handleRequest(w http.ResponseWriter, r *http.Request) {
	sb.mu.RLock()
	isHealthy := sb.healthy
	sb.mu.RUnlock()

	if !isHealthy {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Backend %s is UNHEALTHY (Simulated Failure)\n", sb.Addr)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hello from backend server running at %s! Request Path: %s\n", sb.Addr, r.URL.Path)
}

// handleHealth reports health status to the health checker.
func (sb *SimulatedBackend) handleHealth(w http.ResponseWriter, r *http.Request) {
	sb.mu.RLock()
	isHealthy := sb.healthy
	sb.mu.RUnlock()

	if isHealthy {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "FAIL")
	}
}

// handleToggle allows manual simulation of failures via curl/browser.
func (sb *SimulatedBackend) handleToggle(w http.ResponseWriter, r *http.Request) {
	sb.mu.Lock()
	sb.healthy = !sb.healthy
	currentStatus := sb.healthy
	sb.mu.Unlock()

	statusStr := "HEALTHY"
	if !currentStatus {
		statusStr = "UNHEALTHY"
	}

	log.Printf("[Backend %s] Health manually toggled to: %s", sb.Addr, statusStr)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Backend %s status toggled to %s\n", sb.Addr, statusStr)
}
