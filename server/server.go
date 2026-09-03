package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"letsgolang/health"
)

// GatewayServer manages the lifecycle of the load-balanced reverse proxy gateway.
type GatewayServer struct {
	httpServer      *http.Server
	checker         *health.HealthChecker
	shutdownTimeout time.Duration
}

// NewGatewayServer initializes a new gateway server instance.
func NewGatewayServer(addr string, handler http.Handler, checker *health.HealthChecker, shutdownTimeout time.Duration) *GatewayServer {
	return &GatewayServer{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		checker:         checker,
		shutdownTimeout: shutdownTimeout,
	}
}

// Start launches the server, starts health checks, and blocks waiting for OS signals to shutdown gracefully.
func (gs *GatewayServer) Start() error {
	// 1. Start the active background health checker
	gs.checker.Start()

	// 2. Setup channel to receive HTTP server execution errors (e.g., port already bound)
	serverErrors := make(chan error, 1)

	// 3. Start listening for incoming client connections concurrently
	go func() {
		log.Printf("[GatewayServer] Gateway listening on %s", gs.httpServer.Addr)
		if err := gs.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 4. Register signal handler channel for OS termination signals (SIGINT, SIGTERM)
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	// 5. Block until either the server crashes or we receive an interrupt signal
	select {
	case err := <-serverErrors:
		return err
	case sig := <-shutdownSignal:
		log.Printf("[GatewayServer] Shutdown signal (%v) received. Initiating graceful shutdown...", sig)

		// Create a bounded context to enforce a maximum wait time for active connection draining
		ctx, cancel := context.WithTimeout(context.Background(), gs.shutdownTimeout)
		defer cancel()

		// Stop the background health checking ticker to prevent goroutine leaks
		gs.checker.Stop()

		// Gracefully drain all active HTTP requests. New requests will be rejected immediately,
		// but existing client connections are given until the context expires to complete.
		if err := gs.httpServer.Shutdown(ctx); err != nil {
			log.Printf("[GatewayServer] Graceful shutdown failed: %v. Forcing immediate close.", err)
			_ = gs.httpServer.Close()
			return err
		}

		log.Println("[GatewayServer] Server stopped gracefully.")
		return nil
	}
}
