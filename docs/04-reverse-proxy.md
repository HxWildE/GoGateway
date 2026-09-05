# 04. Reverse Proxy Mechanics and Go's httputil

## 1. CONCEPT
A **Reverse Proxy** stands in front of one or more backend servers, intercepting incoming requests from clients and routing them to a backend. From the client's perspective, the reverse proxy *is* the web server; the client does not know the downstream backends exist.

---

## 2. WHY IT EXISTS
* **Security**: Hides the IP addresses and structures of backend servers, exposing only a single entry point.
* **Load Balancing**: Distributes incoming client traffic across multiple servers.
* **SSL Termination**: Decrypts SSL/TLS traffic at the proxy level so that backend servers do not have to perform intensive cryptographic operations.
* **Caching & Compression**: Caches static content and compresses responses to save backend resources.

---

## 3. HOW IT WORKS
The flow of an HTTP request through a reverse proxy involves:

```
[Client] ──Request──► [Reverse Proxy] ──Modified Request──► [Backend]
  │                     │ (rewrites headers)                  │
  │                     │ (resolves backend IP)               │
  │                     ▼                                     ▼
[Client] ◄─Response─── [Reverse Proxy] ◄─────Response─────── [Backend]
                        (flushes bytes)
```

1. **Intercept**: The proxy accepts the incoming request.
2. **Rewrite (Director)**: The proxy copies the request headers, modifies the `Host` header, sets protocol headers, and updates the URL path to point to the backend server.
3. **Dispatch**: The proxy forwards the modified request over a TCP socket to the backend.
4. **Buffer Response**: The proxy reads the response headers and body from the backend.
5. **Flush**: The proxy writes the headers and stream-flushes the response body back to the client.

---

## 4. INTERNALS: UNDERSTANDING `httputil.ReverseProxy`
We use Go's standard library `httputil.ReverseProxy` struct. It is not magic; it implements `http.Handler` and runs the following steps:

1. **Clones the incoming request**: Creates a shallow copy of `http.Request`.
2. **Runs the `Director`**: Modifies the cloned request (updating the `URL` scheme, host, path, and headers).
3. **Executes `Transport.RoundTrip`**: Forwards the request and retrieves the response. This is done using a connection pool.
4. **Copies response headers**: Copies headers from the backend response (`http.Response`) to the client's response writer (`http.ResponseWriter`).
5. **Flushes the body**: Copies the response body stream using `io.CopyBuffer` to write chunks of bytes back to the client.
6. **Handles errors**: If connection fails, it invokes the custom `ErrorHandler`.

### What the Standard Library Provides:
* Shallow request copying, response body streaming (`io.Copy`), connection pooling (`Transport`), chunked transfer encoding parsing, and HTTP/2 support.

### What We Implement Ourselves:
* Selection of target URLs (load balancing), active/passive failure hook callbacks, custom headers (`X-Forwarded-*`), and gateway routing decisions.

---

## 5. PROJECT USAGE
In `proxy/proxy.go`, we instantiate `httputil.NewSingleHostReverseProxy` for each backend. We customize the `Director` to set `X-Forwarded-For`, `X-Forwarded-Host`, and `X-Forwarded-Proto` and set the `ErrorHandler` to catch network failures.

---

## 6. CODE WALKTHROUGH
```go
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func CreateSingleProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customizing the Director to add tracing headers
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req) // Modifies req.URL to point to target
		req.Header.Set("X-Forwarded-Host", req.Host) // Set original host
		req.Header.Set("Via", "1.1 letsgo-gateway") // Audit trail
	}

	return proxy
}
```

---

## 7. RUNTIME FLOW
```
Client                      Gateway Handler               httputil.ReverseProxy
  │                               │                               │
  ├─ GET /api/users ─────────────►│                               │
  │                               ├─ NextBackend() ──────────────►│
  │                               │  (picks Backend A)            │
  │                               ├──────────────────────────────►│ ServeHTTP(w, r)
  │                               │                               │ ──┐
  │                               │                               │   │ 1. Clone req
  │                               │                               │   │ 2. Run Director
  │                               │                               │   │ 3. RoundTrip() to backend
  │                               │                               │ ◄─┘
  │◄─ Writes HTTP 200 OK ─────────┼───────────────────────────────┤
  │   with streamed users body    │                               │
```

---

## 8. FAILURE CASES
* **Broken Pipeline / Client Disconnects**: If a client closes their browser mid-request, writing data to the client fails. `httputil.ReverseProxy` handles this by checking if the client has disconnected (using `context.Context` cancellation) and stops reading from the backend.
* **Backend Timeout**: If the backend accepts the connection but hangs before responding, the client request will hang.
  * *Code Mitigation*: We customize the `http.Transport` timeouts (`DialContext.Timeout`) to ensure the proxy aborts the wait and runs `ErrorHandler` quickly.

---

## 9. TRADEOFFS
### Custom Proxy Implementation vs. `httputil.ReverseProxy`
* **Using `httputil.ReverseProxy`**:
  * *Pros*: Out-of-the-box support for complex HTTP features like chunked transfers, upgrade headers (WebSockets), HTTP/2, buffer flushing, and context propagation. Less code to write and defend.
  * *Cons*: Customizing deep behaviors (like custom caching or body rewriting) is difficult because the inner request/response loop is closed.
* **Writing Custom TCP Forwarding**:
  * *Pros*: Infinite control over the bytes moving across connections; can build specialized protocols.
  * *Cons*: Requires manually re-implementing HTTP request/response parsers, chunking, keep-alives, connection pools, and error handling. Extremely high risk of security and concurrency bugs.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: What is the difference between a Forward Proxy and a Reverse Proxy?
   * **A**: A **Forward Proxy** acts on behalf of the client (e.g., inside an office network to block sites or hide client IPs). A **Reverse Proxy** acts on behalf of the server (e.g., in a data center to load balance requests and protect backend services).
2. **Q**: Why are headers like `X-Forwarded-For` necessary in a reverse proxy?
   * **A**: Because the backend server establishes a TCP connection directly with the proxy, not the client. As a result, `req.RemoteAddr` on the backend will point to the proxy's IP. The proxy must inject the client's original IP into the `X-Forwarded-For` header so the backend knows who initiated the request.
3. **Q**: How does `httputil.ReverseProxy` handle response streaming?
   * **A**: It uses `io.CopyBuffer` to read data from the backend response body and write it to the client's `ResponseWriter`. If the response is large, it does not load the entire payload into memory. Instead, it streams it in chunks, flushing periodically so the client receives data in real-time.
