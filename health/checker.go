package health

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"letsgolang/loadbalancer"
)

// HealthChecker handles active background polling of downstream backends.
type HealthChecker struct {
	pool      *loadbalancer.BackendPool
	interval  time.Duration
	timeout   time.Duration
	client    *http.Client
	stopChan  chan struct{}
	wg        sync.WaitGroup
	runningMu sync.Mutex
	running   bool
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(pool *loadbalancer.BackendPool, interval, timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		pool:     pool,
		interval: interval,
		timeout:  timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		stopChan: make(chan struct{}),
	}
}

// Start spawns the background polling loop.
func (hc *HealthChecker) Start() {
	hc.runningMu.Lock()
	if hc.running {
		hc.runningMu.Unlock()
		return
	}
	hc.running = true
	hc.runningMu.Unlock()

	hc.wg.Add(1)
	go func() {
		defer hc.wg.Done()
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()

		// Run an initial health check immediately on startup
		hc.checkAll()

		for {
			select {
			case <-ticker.C:
				hc.checkAll()
			case <-hc.stopChan:
				log.Println("[HealthChecker] Stopping active background checks...")
				return
			}
		}
	}()
}

// Stop gracefully shuts down the health checking ticker and waits for any active queries to exit.
func (hc *HealthChecker) Stop() {
	hc.runningMu.Lock()
	if !hc.running {
		hc.runningMu.Unlock()
		return
	}
	hc.running = false
	close(hc.stopChan)
	hc.runningMu.Unlock()

	hc.wg.Wait()
	log.Println("[HealthChecker] Stopped successfully.")
}

// checkAll queries every backend registered in the pool concurrently.
func (hc *HealthChecker) checkAll() {
	backends := hc.pool.GetBackends()
	var checkWg sync.WaitGroup

	for _, b := range backends {
		checkWg.Add(1)
		go func(backend *loadbalancer.Backend) {
			defer checkWg.Done()
			hc.checkBackend(backend)
		}(b)
	}

	checkWg.Wait()
}

// checkBackend sends an HTTP health request to a specific backend's health endpoint.
func (hc *HealthChecker) checkBackend(b *loadbalancer.Backend) {
	healthURL := *b.URL
	healthURL.Path = "/health"

	req, err := http.NewRequestWithContext(context.Background(), "GET", healthURL.String(), nil)
	if err != nil {
		log.Printf("[HealthChecker] Failed to create request for %s: %v", b.URL.Host, err)
		b.SetAlive(false)
		return
	}

	resp, err := hc.client.Do(req)
	if err != nil {
		// Log the transition from alive to dead to avoid log spam, but here we can just log failures
		wasAlive := b.IsAlive()
		b.SetAlive(false)
		if wasAlive {
			log.Printf("[HealthChecker] Backend %s went OFFLINE (Error: %v)", b.URL.Host, err)
		}
		return
	}
	defer resp.Body.Close()

	isHealthy := resp.StatusCode == http.StatusOK
	wasAlive := b.IsAlive()
	b.SetAlive(isHealthy)

	if isHealthy && !wasAlive {
		log.Printf("[HealthChecker] Backend %s went ONLINE (Successfully recovered)", b.URL.Host)
	} else if !isHealthy && wasAlive {
		log.Printf("[HealthChecker] Backend %s went OFFLINE (HTTP status: %d)", b.URL.Host, resp.StatusCode)
	}
}
