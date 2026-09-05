# 02. Go HTTP Server Architecture

## 1. CONCEPT
In Go, an HTTP server is built around the `http.Server` struct, listeners, multiplexers, and handlers. The foundation of Go's high-concurrency model is that the server spawns a new **goroutine** for every incoming TCP connection.

---

## 2. WHY IT EXISTS
HTTP is a application-layer protocol designed to handle request-response cycles. A server must:
1. Bind to a port and listen for incoming TCP connections.
2. Accept connections.
3. Parse the HTTP request text into a structured Go object (`http.Request`).
4. Route the request to the correct handler code.
5. Send back a formatted HTTP response (`http.ResponseWriter`).

Go's standard library provides a highly concurrent, fully featured, and highly performant implementation of this workflow without needing external web frameworks.

---

## 3. HOW IT WORKS
The server runtime follows a simple loop:

```
[ Listener Socket ] ◄── binds to Port (e.g., :8080)
         │
         ▼
    [ Accept() ] ◄── Blocks until Client connects
         │
         ├───────────────────────┐  (Spawns new Goroutine for connection)
         ▼                       ▼
   [ Goroutine 1 ]         [ Goroutine 2 ]
   Read Request            Read Request
   Route through Mux       Route through Mux
   ServeHTTP()             ServeHTTP()
```

1. **`net.Listen`** binds a TCP socket to an address/port.
2. **`Server.ListenAndServe`** loops infinitely, calling `Listener.Accept()` to receive incoming TCP connections.
3. For each accepted connection, `ListenAndServe` launches `go c.serve(connCtx)`—a dedicated goroutine.
4. The goroutine parses the HTTP headers and path, matches them against registered routes in the multiplexer (`http.ServeMux`), and runs the corresponding `http.Handler.ServeHTTP` method.

---

## 4. INTERNALS
* **The Handler Interface**: Everything in Go's HTTP package revolves around this single-method interface:
  ```go
  type Handler interface {
      ServeHTTP(ResponseWriter, *Request)
  }
  ```
* **Mux Matching**: `http.ServeMux` is a multiplexer. It stores a map of routing patterns to handlers. It matches incoming request paths using a prefix match. The longest matching pattern wins.
* **Connection Draining**: When you call `Shutdown()`, the server closes the active listeners to stop accepting new connections, then waits for all outstanding request-handling goroutines to complete their execution before exiting.

---

## 5. PROJECT USAGE
We use `net/http` in two roles:
1. **Backends**: Our simulated backend servers use `http.NewServeMux` and `http.Server` to listen on separate ports (like `8081`, `8082`) and respond with custom text.
2. **Gateway**: Our reverse proxy acts as an HTTP server listening on `:8080`. Its handler (`GatewayHandler`) implements `http.Handler` and intercepts requests, selecting a backend to forward to.

---

## 6. CODE WALKTHROUGH
Our backend server bootstrap matches this pattern:

```go
package backend

import (
	"fmt"
	"net/http"
)

func StartBackendServer(addr string) {
	mux := http.NewServeMux() // Create route multiplexer

	// Register path route
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Response from %s\n", addr)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux, // Pass mux as the root handler
	}

	// Starts listener socket and runs accept loop
	_ = server.ListenAndServe() 
}
```

---

## 7. RUNTIME FLOW
```
Client                      Gateway Server                Downstream Backend
  │                               │                               │
  ├─────── TCP Handshake ────────►│                               │
  │                               │                               │
  ├─────── HTTP Request ─────────►│  (spawns goroutine)           │
  │        "GET /index.html"      │  selects backend              │
  │                               ├─────── HTTP Request ─────────►│
  │                               │        "GET /index.html"      │
  │                               │                               │
  │                               │◄────── HTTP Response ─────────┤
  │                               │        200 OK                 │
  │◄────── HTTP Response ─────────┤                               │
  │        200 OK                 │                               │
```

---

## 8. FAILURE CASES
* **Port Already in Use**: If another process is listening on the target port, `ListenAndServe` returns a socket binding error: `listen tcp :8080: bind: address already in use`.
  * *Code Mitigation*: In `server.Start()`, we capture this error on a channel and trigger a safe exit instead of letting the application crash or hang silently.
* **Slow Clients / Slow Loris Attack**: If a client opens a connection but sends data extremely slowly, it can consume a goroutine and keep a socket open indefinitely, exhausting system descriptors.
  * *Mitigation*: In production, always set `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` on `http.Server`.

---

## 9. TRADEOFFS
### Standard `net/http` vs. Performance Web Frameworks (e.g. Gin, Fiber)
* **Standard `net/http`**:
  * *Pros*: Zero external dependencies, extremely stable, well-maintained, standard interface compatible with the whole Go ecosystem, excellent performance.
  * *Cons*: The default multiplexer has historically had limited support for regex routes or path parameter parsing (though greatly improved in Go 1.22+).
* **Fiber / Fasthttp**:
  * *Pros*: Faster allocations by reusing request contexts (using a goroutine pool and object pooling).
  * *Cons*: Bypasses standard `net/http` interfaces, making integration with standard middlewares difficult; unsafe if request parameters are kept past handler lifetimes.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: How does Go handle thousands of concurrent HTTP connections?
   * **A**: Go uses a **goroutine-per-connection** model. Unlike OS threads, goroutines are extremely lightweight (starting with a ~2KB stack). Go's runtime uses an internal multiplexer (network poller) based on OS primitives (`epoll`/`kqueue`/`IOCP`) to park idle goroutines without blocking OS threads, allowing a single server to scale to millions of concurrent connections.
2. **Q**: What is an `http.Handler`?
   * **A**: An `http.Handler` is any type that implements the `Handler` interface containing a single method: `ServeHTTP(w ResponseWriter, r *Request)`.
3. **Q**: Why should we avoid using `http.ListenAndServe(addr, nil)` in production?
   * **A**: Passing `nil` as the second argument uses the global `http.DefaultServeMux`, which can lead to route hijacking if third-party packages register paths on it. Additionally, default servers have no timeouts configured, making them vulnerable to connection exhaustion attacks.
