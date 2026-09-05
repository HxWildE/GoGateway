# 10. Failure Handling and Edge Cases

## 1. CONCEPT
In systems engineering, networks and downstream services are assumed to be unreliable. A robust reverse proxy must handle various failures gracefully to preserve overall gateway availability.

---

## 2. WHY IT EXISTS
We must analyze and address critical system failure cases. Below is the mapping of common failures, why they happen, and how we handle them.

---

## 3. CORE FAILURE SCENARIOS

### Scenario 1: Backend A is Unavailable / Offline
* **What Happens**: TCP connection attempts to Backend A fail (Connection Refused / Timeout).
* **Why**: The process crashed, is restarting, or is blocked by network issues.
* **How Our Code Handles It**:
  * *Passive detection*: When the proxy tries to route a client request to Backend A and fails, the custom `ErrorHandler` in `proxy.go` is invoked. It runs the callback to set `BackendA.Alive = false` immediately, preventing new traffic from routing to it, and returns `502 Bad Gateway` to the client.
  * *Active detection*: The background health checker pings Backend A, receives a failure, and sets `BackendA.Alive = false` (or keeps it false).
* **Production vs. Us**: A production system would try to **retry** the request on another backend (e.g. Backend B) transparently, so the client never sees a `502 Bad Gateway`.

---

### Scenario 2: All Backends Unavailable
* **What Happens**: The gateway cannot find any healthy downstream server.
* **Why**: System-wide outages, mass deployment failures, or misconfigured backend URLs.
* **How Our Code Handles It**:
  * `NextBackend()` loops through all registered backends and finds none with `Alive == true`, returning `nil`.
  * `GatewayHandler.ServeHTTP` checks if the selected backend is `nil`. If so, it logs the failure and writes an HTTP `503 Service Unavailable` response back to the client.
* **Production vs. Us**: A production server might serve a static, cached "Oops, something went wrong" maintenance page or use a fallback server in a disaster recovery site.

---

### Scenario 3: Backend Recovers / Re-introduced
* **What Happens**: A previously offline backend starts responding to traffic.
* **Why**: The process restarted or the network partition resolved.
* **How Our Code Handles It**:
  * The background `HealthChecker` loop continues pinging the offline backend.
  * When the backend responds with HTTP 200 "OK", `HealthChecker` logs the recovery and calls `backend.SetAlive(true)`.
  * The next execution of `NextBackend()` immediately includes it in the round-robin rotation.
* **Production vs. Us**: Similar, but production systems might employ a "warm-up" period where traffic is introduced gradually rather than sending 50% of the load immediately (which can cause the backend to crash again).

---

### Scenario 4: Proxy Receives Malformed/Invalid Requests
* **What Happens**: A client sends invalid HTTP headers, bad chunked sizes, or large payloads.
* **Why**: Client-side bugs, slow network links corrupting packets, or security scanners.
* **How Our Code Handles It**:
  * The Go standard library `net/http` server automatically parses request structures. If a request violates HTTP syntax, the server rejects it early, returning HTTP `400 Bad Request` before calling our `GatewayHandler`.
* **Production vs. Us**: Production systems have strict rules for maximum header size (`MaxHeaderBytes`), max request body size, and request rate limits to block malicious DoS attacks.

---

### Scenario 5: Backend Responds Slowly (Slow Backend)
* **What Happens**: The backend accepts the TCP connection but takes 30 seconds to reply.
* **Why**: Heavy database queries, deadlock, CPU saturation, or memory thrashing.
* **How Our Code Handles It**:
  * We configure the `http.Transport` inside the proxy with a `DialContext.Timeout` and run requests with the client context.
* **Production vs. Us**: Production systems use **Circuit Breakers**. If a backend's latency or error rate exceeds a threshold over a sliding window, the circuit breaker trips, instantly failing all requests to that backend for a cooling-off period to let it recover.

---

### Scenario 6: Concurrent Access to Backend Pool State
* **What Happens**: Dozens of requests read the pool state while the health checker modifies it.
* **Why**: High traffic concurrency.
* **How Our Code Handles It**:
  * We use `sync.RWMutex` to guard the `Alive` status inside `Backend` and `sync/atomic` for `BackendPool.current`. This prevents data races and memory corruption.
* **Production vs. Us**: Similar, but production proxies are often multi-threaded processes using lock-free data structures or event loops (like NGINX's single-threaded worker process) to minimize lock contention.

---

### Scenario 7: Proxy Shutdown During Active Requests
* **What Happens**: The proxy gateway process is terminated while client requests are being processed.
* **Why**: Deployment rolling updates or administrator restarts.
* **How Our Code Handles It**:
  * We listen for `SIGINT` / `SIGTERM` signals. Upon receiving them, we trigger `http.Server.Shutdown()`.
  * This closes all listener sockets (preventing new requests) and waits up to 10 seconds for current request goroutines to finish executing.
* **Production vs. Us**: Production servers might wait longer (e.g. 60 seconds) or notify upstream load balancers (e.g., DNS, AWS ALB) to stop routing traffic to this node first.

---

### Scenario 8: Health Checker Running During Shutdown
* **What Happens**: The background health check loop tries to query nodes while the gateway is shutting down.
* **Why**: Improper cleanup order.
* **How Our Code Handles It**:
  * During shutdown, we stop the health checker first (`checker.Stop()`). This closes the `stopChan`, stops the ticker, and blocks using a `sync.WaitGroup` until all active query goroutines complete. Only then do we shut down the gateway HTTP server.
* **Production vs. Us**: Identical in design.

---

## 4. INTERVIEW QUESTIONS
1. **Q**: What is the difference between active and passive failure detection?
   * **A**: Active failure detection (health checking) uses background queries to verify status. Passive failure detection monitors inline client traffic. If a proxy fails to route a client's request to a backend, it marks the backend dead immediately, reacting faster than periodic active polling.
2. **Q**: How does a circuit breaker pattern work in systems engineering?
   * **A**: A circuit breaker is a proxy wrapper that tracks failures. It has three states: **Closed** (traffic flows normally), **Open** (errors are high; requests fail immediately without hitting the backend), and **Half-Open** (routes a trial request to check if the backend has recovered). It prevents cascading failures.
3. **Q**: What happens if a client disconnects while the proxy is waiting for the backend to respond?
   * **A**: Go's standard library propagates the client's context cancellation. The `httputil.ReverseProxy` detects that the request context has been cancelled, terminates the backend connection, and frees resources immediately.
