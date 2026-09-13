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

// Start launches the server, starts health checks, and blocks for graceful shutdown.
func (gs *GatewayServer) Start() error {
	gs.checker.Start()

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("[GatewayServer] Gateway listening on %s", gs.httpServer.Addr)
		if err := gs.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return err
	case sig := <-shutdownSignal:
		log.Printf("[GatewayServer] Shutdown signal (%v) received. Initiating graceful shutdown...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), gs.shutdownTimeout)
		defer cancel()

		gs.checker.Stop()

		if err := gs.httpServer.Shutdown(ctx); err != nil {
			log.Printf("[GatewayServer] Graceful shutdown failed: %v. Forcing immediate close.", err)
			_ = gs.httpServer.Close()
			return err
		}

		log.Println("[GatewayServer] Server stopped gracefully.")
		return nil
	}
}
