package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"letsgolang/loadbalancer"
	"letsgolang/proxy"
)

func TestGatewayIntegration(t *testing.T) {
	// 1. Start two test backend servers with distinct IDs
	backendASrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend-A"))
	}))
	defer backendASrv.Close()

	backendBSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend-B"))
	}))
	defer backendBSrv.Close()

	urlA, _ := url.Parse(backendASrv.URL)
	urlB, _ := url.Parse(backendBSrv.URL)

	// 2. Initialize Backend Pool and proxies
	pool := loadbalancer.NewBackendPool()

	var bA, bB *loadbalancer.Backend

	proxyA := proxy.NewProxy(urlA, time.Second, func(err error) {
		if bA != nil {
			bA.SetAlive(false)
		}
	})

	proxyB := proxy.NewProxy(urlB, time.Second, func(err error) {
		if bB != nil {
			bB.SetAlive(false)
		}
	})

	bA = &loadbalancer.Backend{URL: urlA, Alive: true, ReverseProxy: proxyA}
	bB = &loadbalancer.Backend{URL: urlB, Alive: true, ReverseProxy: proxyB}

	pool.AddBackend(bA)
	pool.AddBackend(bB)

	// 3. Setup Gateway Handler
	gatewayHandler := proxy.NewGatewayHandler(pool)

	// Helper function to query the gateway
	queryGateway := func() string {
		req := httptest.NewRequest("GET", "http://gateway.local/", nil)
		rec := httptest.NewRecorder()
		gatewayHandler.ServeHTTP(rec, req)
		resp := rec.Result()
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	// 4. Test normal round-robin (distribution among A and B)
	// Since current starts at 0, atomic.Add sets it to 1 -> index 1 -> Backend B
	// Next is index 2 -> 0 -> Backend A
	firstResp := queryGateway()
	secondResp := queryGateway()

	if firstResp == secondResp {
		t.Errorf("Expected different responses from successive requests, but got both %q", firstResp)
	}

	// Ensure both are represented
	resps := map[string]bool{firstResp: true, secondResp: true}
	if !resps["backend-A"] || !resps["backend-B"] {
		t.Errorf("Expected to hit both backend-A and backend-B, but got: %v", resps)
	}

	// 5. Simulate Backend A going offline
	bA.SetAlive(false)

	// Query gateway 4 times; all responses must go to Backend B now
	for i := 0; i < 4; i++ {
		resp := queryGateway()
		if resp != "backend-B" {
			t.Errorf("Expected only backend-B while backend-A is dead, but got %q", resp)
		}
	}

	// 6. Simulate Backend A recovery
	bA.SetAlive(true)

	// Query gateway 4 times; traffic should distribute between A and B again
	responses := make(map[string]int)
	for i := 0; i < 10; i++ {
		responses[queryGateway()]++
	}

	if responses["backend-A"] == 0 || responses["backend-B"] == 0 {
		t.Errorf("Expected traffic to resume routing to both after recovery, got distributions: %v", responses)
	}
}
