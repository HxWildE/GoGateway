package loadbalancer

import (
	"net/http/httputil"
	"net/url"
	"sync"
	"sync/atomic"
)

// Backend represents a single downstream backend server.
type Backend struct {
	URL          *url.URL
	Alive        bool
	ReverseProxy *httputil.ReverseProxy
	mu           sync.RWMutex // Protects the Alive field
}

// SetAlive thread-safely updates the health state of the backend.
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	b.Alive = alive
	b.mu.Unlock()
}

// IsAlive thread-safely checks if the backend is currently marked healthy.
func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	alive := b.Alive
	b.mu.RUnlock()
	return alive
}

// BackendPool orchestrates a set of backend instances and routes traffic among them.
type BackendPool struct {
	backends []*Backend
	current  uint64 // Index used for the round-robin selection (accessed atomically)
}

// NewBackendPool initializes a new pool of backends.
func NewBackendPool() *BackendPool {
	return &BackendPool{
		backends: make([]*Backend, 0),
		current:  0,
	}
}

// AddBackend registers a new backend server into the pool.
func (bp *BackendPool) AddBackend(b *Backend) {
	bp.backends = append(bp.backends, b)
}

// GetBackends returns all registered backend instances in the pool.
func (bp *BackendPool) GetBackends() []*Backend {
	return bp.backends
}

// NextBackend performs a thread-safe, atomic Round-Robin search to select the next healthy backend.
// It loops through all backends at most once. If all backends are unhealthy, it returns nil.
func (bp *BackendPool) NextBackend() *Backend {
	n := uint64(len(bp.backends))
	if n == 0 {
		return nil
	}

	// Increment the counter atomically to avoid race conditions.
	// The return value is a strictly increasing index.
	next := atomic.AddUint64(&bp.current, 1)

	// Search up to n times to find the first healthy backend starting from our round-robin index.
	for i := uint64(0); i < n; i++ {
		idx := (next + i) % n
		candidate := bp.backends[idx]
		if candidate.IsAlive() {
			return candidate
		}
	}

	return nil
}
