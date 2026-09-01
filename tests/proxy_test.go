package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"letsgolang/proxy"
)

func TestProxyForwardingAndHeaders(t *testing.T) {
	var receivedHeaders http.Header

	// 1. Create a simulated backend server using httptest
	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("backend success"))
	}))
	defer backendSrv.Close()

	backendURL, err := url.Parse(backendSrv.URL)
	if err != nil {
		t.Fatalf("Failed to parse backend URL: %v", err)
	}

	// 2. Initialize our proxy handler pointing to this backend
	var failedCount int64
	proxyHandler := proxy.NewProxy(backendURL, 2*time.Second, func(err error) {
		atomic.AddInt64(&failedCount, 1)
	})

	// 3. Create a test client request hitting our proxy handler
	req := httptest.NewRequest("GET", "http://gateway.local/some-path", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	req.Host = "gateway.local"

	rec := httptest.NewRecorder()

	// 4. Run the proxy handler
	proxyHandler.ServeHTTP(rec, req)

	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// 5. Verify the response and headers modified by proxy
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	if string(body) != "backend success" {
		t.Errorf("Expected response body 'backend success', got %q", string(body))
	}

	// Check forwarded headers
	forwardedFor := receivedHeaders.Get("X-Forwarded-For")
	if forwardedFor != "192.168.1.100" {
		t.Errorf("Expected X-Forwarded-For to be '192.168.1.100', got %q", forwardedFor)
	}

	forwardedHost := receivedHeaders.Get("X-Forwarded-Host")
	if forwardedHost != "gateway.local" {
		t.Errorf("Expected X-Forwarded-Host to be 'gateway.local', got %q", forwardedHost)
	}

	forwardedProto := receivedHeaders.Get("X-Forwarded-Proto")
	if forwardedProto != "http" {
		t.Errorf("Expected X-Forwarded-Proto to be 'http', got %q", forwardedProto)
	}

	if atomic.LoadInt64(&failedCount) != 0 {
		t.Error("Failure callback was triggered unexpectedly")
	}
}

func TestProxyPassiveFailureTrigger(t *testing.T) {
	// 1. Create a simulated backend and close it immediately to force connection failure
	backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	backendURL, _ := url.Parse(backendSrv.URL)
	backendSrv.Close() // Backend is offline now

	// 2. Set up our proxy with a passive failure callback
	var failedCount int64
	proxyHandler := proxy.NewProxy(backendURL, 100*time.Millisecond, func(err error) {
		atomic.AddInt64(&failedCount, 1)
	})

	// 3. Issue a request through proxy handler
	req := httptest.NewRequest("GET", "http://gateway.local/path", nil)
	rec := httptest.NewRecorder()
	proxyHandler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	// 4. Verify that we got a 502 Bad Gateway and the failure callback triggered
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("Expected status 502 Bad Gateway, got %d", resp.StatusCode)
	}

	if atomic.LoadInt64(&failedCount) != 1 {
		t.Errorf("Expected failure callback count to be 1, got %d", atomic.LoadInt64(&failedCount))
	}
}
