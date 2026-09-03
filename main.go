package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"letsgolang/backend"
	"letsgolang/config"
	"letsgolang/health"
	"letsgolang/loadbalancer"
	"letsgolang/proxy"
	"letsgolang/server"
)

func main() {
	log.Println("[Main] Initializing Gateway and Backends...")

	// 1. Load configuration from flags
	cfg := config.LoadConfig()

	// 2. Start simulated backends in concurrent goroutines
	var simulatedServers []*backend.SimulatedBackend
	for _, addr := range cfg.BackendAddrs {
		sb := backend.NewSimulatedBackend(addr)
		simulatedServers = append(simulatedServers, sb)

		// Start backend in a background goroutine
		go func(srv *backend.SimulatedBackend) {
			if err := srv.Start(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[Main] Simulated backend at %s failed: %v", srv.Addr, err)
			}  
		}(sb)
	}
	//If a err is thrown and its not http.ErrServerClosed (HEALHTY GRACEFUL EXIT ERR) ,log Fatal 

	// Defer cleanup of simulated backends when main exits
	defer func() {
		log.Println("[Main] Cleaning up simulated backends...")
		for _, srv := range simulatedServers {
			_ = srv.Close()
		}
	}()

	// Give simulated backends a brief moment to start up before checking health
	time.Sleep(500 * time.Millisecond)

	// 3. Initialize Backend Pool
	pool := loadbalancer.NewBackendPool()
	for _, addr := range cfg.BackendAddrs {
		backendURL, err := url.Parse(fmt.Sprintf("http://%s", addr))
		if err != nil {
			log.Fatalf("[Main] Invalid backend URL %s: %v", addr, err)
		}

		// Closure-based passive failure detection:
		// We define the Backend pointer first so the error handler can close over it.
		var b *loadbalancer.Backend

		proxyHandler := proxy.NewProxy(backendURL, cfg.ProxyTimeout, func(err error) {
			if b != nil {
				// Mark backend offline immediately upon request routing failure
				b.SetAlive(false)
			}
		})

		b = &loadbalancer.Backend{
			URL:          backendURL,
			Alive:        true, // Start by assuming healthy; health checker will verify
			ReverseProxy: proxyHandler,
		}

		pool.AddBackend(b)
	}

	// 4. Create active background Health Checker
	checker := health.NewHealthChecker(pool, cfg.HealthCheckInterval, cfg.HealthCheckTimeout)

	// 5. Create reverse proxy Gateway HTTP Handler
	gatewayHandler := proxy.NewGatewayHandler(pool)

	// 6. Create and start Orchestrator Server
	srv := server.NewGatewayServer(cfg.GatewayAddr, gatewayHandler, checker, 10*time.Second)

	log.Printf("[Main] System initialized. Routing to: %v", cfg.BackendAddrs)
	if err := srv.Start(); err != nil {
		log.Fatalf("[Main] Gateway encountered critical error: %v", err)
	}
}
