package proxy

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"letsgolang/loadbalancer"
)

// NewProxy creates a configured ReverseProxy instance for a specific backend URL.
// It uses a custom transport to set timeouts and an ErrorHandler to trigger passive failure detection.
func NewProxy(target *url.URL, timeout time.Duration, onFailure func(error)) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure a dedicated Transport layer with timeouts to manage connection pooling.
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   timeout,         // Time limit to establish a TCP connection
			KeepAlive: 30 * time.Second, // Keep-alive probe interval
		}).DialContext,
		MaxIdleConns:          100,               // Max idle connections across all hosts
		IdleConnTimeout:       90 * time.Second,  // Keep idle connections open for 90s
		TLSHandshakeTimeout:   10 * time.Second,  // Max time allowed for TLS handshakes
		ExpectContinueTimeout: 1 * time.Second,   // Wait time for 100-continue header response
	}

	// Customize the Request Director to modify request headers before forwarding.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// 1. Set X-Forwarded-Host to preserve client's original HTTP Host header
		if req.Header.Get("X-Forwarded-Host") == "" {
			req.Header.Set("X-Forwarded-Host", req.Host)
		}

		// 2. Set X-Forwarded-Proto to track original client protocol schema (HTTP or HTTPS)
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	// Define a custom ErrorHandler. If the proxy fails to reach the backend,
	// it triggers passive failure detection, marking the backend offline immediately.
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[Proxy] Error routing request to %s: %v. Marking backend offline.", target.String(), err)
		onFailure(err)

		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "Bad Gateway: Failed to connect to downstream backend server.\n")
	}

	return proxy
}

// GatewayHandler coordinates incoming client traffic to the appropriate backend using the load balancer.
type GatewayHandler struct {
	Pool *loadbalancer.BackendPool
}

// NewGatewayHandler creates a new reverse proxy gateway routing handler.
func NewGatewayHandler(pool *loadbalancer.BackendPool) *GatewayHandler {
	return &GatewayHandler{
		Pool: pool,
	}
}

// ServeHTTP implements the http.Handler interface to route requests.
func (gh *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Exclude favicon requests from logging and proxying
	if r.URL.Path == "/favicon.ico" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// 1. Select the next healthy backend from the pool
	targetBackend := gh.Pool.NextBackend()
	if targetBackend == nil {
		log.Printf("[Gateway] Request FAILED: No healthy backends available in the pool for path %q", r.URL.Path)
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Service Unavailable: No healthy backend servers are currently active in the pool.\n")
		return
	}

	log.Printf("[Gateway] Routing client %s -> backend %s (%s)", r.RemoteAddr, targetBackend.URL.Host, r.URL.Path)

	// 2. Delegate execution to the backend's ReverseProxy handler
	targetBackend.ReverseProxy.ServeHTTP(w, r)
}
