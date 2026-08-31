package tests

import (
	"net/url"
	"sync"
	"testing"

	"letsgolang/loadbalancer"
)

func TestRoundRobinSelection(t *testing.T) {
	pool := loadbalancer.NewBackendPool()

	urlA, _ := url.Parse("http://127.0.0.1:8081")
	urlB, _ := url.Parse("http://127.0.0.1:8082")
	urlC, _ := url.Parse("http://127.0.0.1:8083")

	bA := &loadbalancer.Backend{URL: urlA, Alive: true}
	bB := &loadbalancer.Backend{URL: urlB, Alive: true}
	bC := &loadbalancer.Backend{URL: urlC, Alive: true}

	pool.AddBackend(bA)
	pool.AddBackend(bB)
	pool.AddBackend(bC)

	// Since current index is initialized to 0, atomic.AddUint64 sets it to 1.
	// 1 % 3 = 1 -> bB
	// 2 % 3 = 2 -> bC
	// 3 % 3 = 0 -> bA
	// Let's verify that successive calls return all three in order.
	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		b := pool.NextBackend()
		if b == nil {
			t.Fatal("Expected backend, got nil")
		}
		seen[b.URL.Host]++
	}

	if seen["127.0.0.1:8081"] != 2 || seen["127.0.0.1:8082"] != 2 || seen["127.0.0.1:8083"] != 2 {
		t.Errorf("Round Robin distribution incorrect. Seen map: %v", seen)
	}
}

func TestUnhealthyBackendExclusion(t *testing.T) {
	pool := loadbalancer.NewBackendPool()

	urlA, _ := url.Parse("http://127.0.0.1:8081")
	urlB, _ := url.Parse("http://127.0.0.1:8082")

	bA := &loadbalancer.Backend{URL: urlA, Alive: true}
	bB := &loadbalancer.Backend{URL: urlB, Alive: false} // Backend B is dead

	pool.AddBackend(bA)
	pool.AddBackend(bB)

	// NextBackend should only return Backend A since B is dead
	for i := 0; i < 5; i++ {
		b := pool.NextBackend()
		if b == nil {
			t.Fatal("Expected active backend A, got nil")
		}
		if b.URL.Host != "127.0.0.1:8081" {
			t.Errorf("Expected backend A, but got %s", b.URL.Host)
		}
	}

	// Make Backend B alive and Backend A dead
	bB.SetAlive(true)
	bA.SetAlive(false)

	// NextBackend should now only return Backend B
	for i := 0; i < 5; i++ {
		b := pool.NextBackend()
		if b == nil {
			t.Fatal("Expected active backend B, got nil")
		}
		if b.URL.Host != "127.0.0.1:8082" {
			t.Errorf("Expected backend B, but got %s", b.URL.Host)
		}
	}

	// Mark both dead
	bB.SetAlive(false)
	b := pool.NextBackend()
	if b != nil {
		t.Errorf("Expected nil when all backends are dead, got %s", b.URL.Host)
	}
}

func TestConcurrentBackendSelection(t *testing.T) {
	pool := loadbalancer.NewBackendPool()

	urlA, _ := url.Parse("http://127.0.0.1:8081")
	urlB, _ := url.Parse("http://127.0.0.1:8082")

	bA := &loadbalancer.Backend{URL: urlA, Alive: true}
	bB := &loadbalancer.Backend{URL: urlB, Alive: true}

	pool.AddBackend(bA)
	pool.AddBackend(bB)

	var wg sync.WaitGroup
	workers := 50
	iterations := 1000

	// Launch multiple goroutines selecting backends simultaneously to verify lack of races
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				b := pool.NextBackend()
				if b == nil {
					t.Error("Expected healthy backend, got nil")
					return
				}
				// Simulate read access
				_ = b.IsAlive()
			}
		}()
	}

	wg.Wait()
}
