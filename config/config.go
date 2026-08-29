package config

import (
	"flag"
	"strings"
	"time"
)

// Config holds the configuration settings for the load balancer gateway and backend simulation.
type Config struct {
	GatewayAddr         string        // Address the gateway server listens on (e.g., ":8080")
	BackendAddrs        []string      // List of downstream backend server addresses (e.g., "127.0.0.1:8081")
	HealthCheckInterval time.Duration // Interval between periodic health checks
	HealthCheckTimeout  time.Duration // Network timeout for health check requests
	ProxyTimeout        time.Duration // Network timeout for reverse proxy client request forwarding
}

// LoadConfig parses command line flags and returns a populated Config struct.
func LoadConfig() *Config {
	gatewayAddr := flag.String("gateway", ":8080", "Address the reverse proxy gateway listens on")
	backendsStr := flag.String("backends", "127.0.0.1:8081,127.0.0.1:8082", "Comma-separated list of backend addresses")
	hcInterval := flag.Duration("health-interval", 5*time.Second, "Interval between backend health checks")
	hcTimeout := flag.Duration("health-timeout", 2*time.Second, "Timeout for health check requests")
	proxyTimeout := flag.Duration("proxy-timeout", 10*time.Second, "Timeout for reverse proxy client request forwarding")

	flag.Parse()

	// Parse comma-separated backend addresses into a slice
	var backendAddrs []string
	for _, addr := range strings.Split(*backendsStr, ",") {
		addr = strings.TrimSpace(addr)
		if addr != "" {
			backendAddrs = append(backendAddrs, addr)
		}
	}

	return &Config{
		GatewayAddr:         *gatewayAddr,
		BackendAddrs:        backendAddrs,
		HealthCheckInterval: *hcInterval,
		HealthCheckTimeout:  *hcTimeout,
		ProxyTimeout:        *proxyTimeout,
	}
}
