# 12. Complete Request Runtime Flow Trace

## 1. CONCEPT
Understanding a network program requires tracing a single client request through the entire stack, mapping functions, goroutines, structs, lock acquisitions, and response pathways.

---

## 2. THE BIG PICTURE — ONE REQUEST, END-TO-END

Before diving into code, here is **the entire journey of a single HTTP request** through our system. Study this diagram until you can redraw it from memory — interviewers love asking "walk me through a request."

```
╔══════════════════════════════════════════════════════════════════════════════════╗
║                          COMPLETE REQUEST LIFECYCLE                            ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                ║
║   ┌──────────┐                                                                 ║
║   │  CLIENT  │  (e.g. curl, browser, another service)                          ║
║   └────┬─────┘                                                                 ║
║        │                                                                       ║
║        │  ① TCP 3-Way Handshake (SYN → SYN-ACK → ACK)                         ║
║        │  ② HTTP Request: "GET /api/users HTTP/1.1"                            ║
║        ▼                                                                       ║
║   ╔════════════════════════════════════════════════════════╗                    ║
║   ║              GATEWAY SERVER (:8080)                   ║                    ║
║   ║                                                       ║                    ║
║   ║   ┌─────────────────────────────────────────────┐     ║                    ║
║   ║   │  net/http Listener Socket                   │     ║                    ║
║   ║   │  ─────────────────────────────              │     ║                    ║
║   ║   │  ③ Accept() → spawns Goroutine G1           │     ║                    ║
║   ║   │  ④ Parse raw bytes → http.Request struct    │     ║                    ║
║   ║   └──────────────────┬──────────────────────────┘     ║                    ║
║   ║                      │                                ║                    ║
║   ║                      ▼                                ║                    ║
║   ║   ┌─────────────────────────────────────────────┐     ║                    ║
║   ║   │  GatewayHandler.ServeHTTP(w, r)   [on G1]  │     ║                    ║
║   ║   │  ─────────────────────────────────          │     ║                    ║
║   ║   │  ⑤ Call Pool.NextBackend()                  │     ║                    ║
║   ║   └──────────────────┬──────────────────────────┘     ║                    ║
║   ║                      │                                ║                    ║
║   ║                      ▼                                ║                    ║
║   ║   ┌─────────────────────────────────────────────┐     ║                    ║
║   ║   │  BackendPool.NextBackend()          [on G1] │     ║                    ║
║   ║   │  ─────────────────────────────────          │     ║                    ║
║   ║   │  ⑥ atomic.AddUint64(&current, 1)           │     ║                    ║
║   ║   │  ⑦ idx = counter % len(backends)           │     ║                    ║
║   ║   │  ⑧ candidate.IsAlive()  ← RLock/RUnlock   │     ║                    ║
║   ║   │  ⑨ Return *Backend pointer (Backend A)     │     ║                    ║
║   ║   └──────────────────┬──────────────────────────┘     ║                    ║
║   ║                      │                                ║                    ║
║   ║                      ▼                                ║                    ║
║   ║   ┌─────────────────────────────────────────────┐     ║                    ║
║   ║   │  httputil.ReverseProxy.ServeHTTP    [on G1] │     ║                    ║
║   ║   │  ─────────────────────────────────          │     ║                    ║
║   ║   │  ⑩ Clone http.Request (shallow copy)       │     ║                    ║
║   ║   │  ⑪ Run Director() → rewrite Host, headers  │     ║                    ║
║   ║   │  ⑫ Transport.RoundTrip() → forward req     │     ║                    ║
║   ║   └──────────────────┬──────────────────────────┘     ║                    ║
║   ╚══════════════════════╪════════════════════════════════╝                    ║
║                          │                                                     ║
║          ════════════════╪═══════════════════  NETWORK BOUNDARY                ║
║            LEG 1 above   │   LEG 2 below                                      ║
║          ════════════════╪═══════════════════                                  ║
║                          │                                                     ║
║                          │  ⑬ Reuse/Create TCP socket from connection pool     ║
║                          │  ⑭ Send rewritten HTTP request bytes                ║
║                          ▼                                                     ║
║   ╔════════════════════════════════════════════════════════╗                    ║
║   ║              BACKEND A (:8081)                        ║                    ║
║   ║                                                       ║                    ║
║   ║   ⑮ Process request → generate response              ║                    ║
║   ║   ⑯ Write HTTP 200 OK + body payload                 ║                    ║
║   ║                                                       ║                    ║
║   ╚══════════════════════╪════════════════════════════════╝                    ║
║                          │                                                     ║
║          ════════════════╪═══════════════════  RESPONSE PATH                   ║
║                          │                                                     ║
║                          ▼                                                     ║
║   ╔════════════════════════════════════════════════════════╗                    ║
║   ║              GATEWAY SERVER (:8080)                   ║                    ║
║   ║                                                       ║                    ║
║   ║   ⑰ Copy response headers → client ResponseWriter    ║                    ║
║   ║   ⑱ Stream body via io.CopyBuffer (chunked)          ║                    ║
║   ║   ⑲ Return TCP socket to connection pool              ║                    ║
║   ║                                                       ║                    ║
║   ╚══════════════════════╪════════════════════════════════╝                    ║
║                          │                                                     ║
║                          ▼                                                     ║
║   ┌──────────┐                                                                 ║
║   │  CLIENT  │  ← receives HTTP 200 OK + streamed body                        ║
║   └──────────┘                                                                 ║
║                                                                                ║
╚══════════════════════════════════════════════════════════════════════════════════╝
```

---

## 3. TWO NETWORK LEGS — THE KEY INTERVIEW INSIGHT

The proxy creates **two independent TCP connections**. This is the most important networking concept to defend:

```
   ┌──────────┐          LEG 1                ┌───────────────┐          LEG 2               ┌────────────┐
   │          │  TCP Socket A                  │               │  TCP Socket B                │            │
   │  CLIENT  │ ◄══════════════════════════► │  GATEWAY      │ ◄════════════════════════► │  BACKEND   │
   │          │  192.168.1.50:54321           │  PROXY        │  127.0.0.1:Random            │  SERVER    │
   │          │  ◄──────────────────►          │  :8080        │  ◄────────────────►          │  :8081     │
   └──────────┘  Client IP : Client Port      └───────────────┘  Proxy IP : Ephemeral Port  └────────────┘

   ◄───────── Client sees ONLY the proxy ─────►  ◄────── Backend sees ONLY the proxy ──────►

   The client does NOT know                      The backend does NOT know
   Backend A exists.                             the client exists.
   It talks to :8080.                            It sees proxy's IP in RemoteAddr.
```

### Why This Matters
* **Security**: Backend IPs are hidden from clients. Attackers can't target them directly.
* **Flexibility**: You can add/remove backends without clients knowing.
* **Headers**: `X-Forwarded-For` exists precisely because Backend A's `RemoteAddr` shows the **proxy's** IP, not the client's.

---

## 4. THE ERROR PATH — WHAT HAPPENS WHEN BACKEND FAILS

```
╔══════════════════════════════════════════════════════════════════╗
║                     FAILURE FLOW                               ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                ║
║   Client ──── GET /api/users ────► Gateway                     ║
║                                      │                         ║
║                                      ▼                         ║
║                              Pool.NextBackend()                ║
║                                      │                         ║
║                                      ▼                         ║
║                              Selects Backend A                 ║
║                                      │                         ║
║                                      ▼                         ║
║                           ReverseProxy.ServeHTTP()             ║
║                                      │                         ║
║                                      ▼                         ║
║                          Transport.RoundTrip()                 ║
║                                      │                         ║
║                                      ▼                         ║
║                      ┌───────────────────────────────┐         ║
║                      │   TCP Connect to :8081        │         ║
║                      │   ─────────────────────       │         ║
║                      │                               │         ║
║                      │   ╔═══════════════════════╗   │         ║
║                      │   ║  CONNECTION REFUSED!  ║   │         ║
║                      │   ║  (Backend is dead)    ║   │         ║
║                      │   ╚═══════════════════════╝   │         ║
║                      │                               │         ║
║                      └───────────────┬───────────────┘         ║
║                                      │                         ║
║                                      ▼                         ║
║                      ┌───────────────────────────────┐         ║
║                      │   proxy.ErrorHandler()        │         ║
║                      │   ─────────────────────       │         ║
║                      │                               │         ║
║                      │   1. Log the failure          │         ║
║                      │   2. Run onFailure closure:   │         ║
║                      │      b.SetAlive(false)        │         ║
║                      │      ├── Lock(b.mu)           │         ║
║                      │      ├── b.Alive = false      │         ║
║                      │      └── Unlock(b.mu)         │         ║
║                      │   3. Write 502 Bad Gateway    │         ║
║                      │                               │         ║
║                      └───────────────┬───────────────┘         ║
║                                      │                         ║
║                                      ▼                         ║
║   Client ◄──── HTTP 502 Bad Gateway ────                       ║
║                                                                ║
║   Meanwhile, next request skips Backend A                      ║
║   because IsAlive() now returns false.                         ║
║                                                                ║
╚══════════════════════════════════════════════════════════════════╝
```

---

## 5. CONCURRENT GOROUTINES — WHO DOES WHAT

This diagram shows every goroutine in the system at runtime and what shared state they touch:

```
╔════════════════════════════════════════════════════════════════════════════╗
║                     GOROUTINE MAP AT RUNTIME                            ║
╠════════════════════════════════════════════════════════════════════════════╣
║                                                                         ║
║   GOROUTINE: main                                                       ║
║   ├── Starts Backend A server goroutine                                 ║
║   ├── Starts Backend B server goroutine                                 ║
║   ├── Creates BackendPool, HealthChecker                                ║
║   └── Calls server.Start() (blocks here)                                ║
║                                                                         ║
║   GOROUTINE: Backend A listener                                         ║
║   └── Accepts connections on :8081 (independent, no shared state)       ║
║                                                                         ║
║   GOROUTINE: Backend B listener                                         ║
║   └── Accepts connections on :8082 (independent, no shared state)       ║
║                                                                         ║
║   GOROUTINE: Health Checker (background)                                ║
║   │   Runs every 5 seconds via time.Ticker                              ║
║   │                                                                     ║
║   │   ┌─────────────── SHARED STATE WRITES ──────────────────┐          ║
║   │   │  backend.SetAlive(true/false)                        │          ║
║   │   │  ├── Acquires backend.mu.Lock()    (exclusive)       │          ║
║   │   │  ├── Writes backend.Alive = true/false               │          ║
║   │   │  └── Releases backend.mu.Unlock()                    │          ║
║   │   └──────────────────────────────────────────────────────┘          ║
║   │                                                                     ║
║   └── Spawns sub-goroutines to ping each backend in parallel            ║
║                                                                         ║
║   GOROUTINE: Request Handler G1 (per-client, spawned by net/http)       ║
║   GOROUTINE: Request Handler G2 (per-client, spawned by net/http)       ║
║   GOROUTINE: Request Handler G3 ...                                     ║
║   │                                                                     ║
║   │   ┌─────────────── SHARED STATE READS ───────────────────┐          ║
║   │   │  pool.NextBackend()                                  │          ║
║   │   │  ├── atomic.AddUint64(&pool.current, 1)  (lock-free) │          ║
║   │   │  ├── candidate.IsAlive()                             │          ║
║   │   │  │   ├── Acquires backend.mu.RLock()  (shared read)  │          ║
║   │   │  │   ├── Reads backend.Alive                         │          ║
║   │   │  │   └── Releases backend.mu.RUnlock()               │          ║
║   │   │  └── Returns *Backend pointer                        │          ║
║   │   └──────────────────────────────────────────────────────┘          ║
║   │                                                                     ║
║   └── On proxy error: calls onFailure closure (WRITE to Alive)          ║
║                                                                         ║
║   GOROUTINE: Signal Listener (inside server.Start)                      ║
║   └── Blocks on <-shutdownSignal, triggers graceful drain               ║
║                                                                         ║
╚════════════════════════════════════════════════════════════════════════════╝


   LOCK COMPATIBILITY MATRIX:
   ┌─────────────────┬──────────────┬──────────────┐
   │                 │  RLock held  │  Lock held   │
   ├─────────────────┼──────────────┼──────────────┤
   │  RLock request  │   ✅ ALLOWED  │  ❌ BLOCKED   │
   │  Lock request   │   ❌ BLOCKED  │  ❌ BLOCKED   │
   └─────────────────┴──────────────┴──────────────┘

   Many request goroutines can read IsAlive() simultaneously.
   Only the health checker (or error handler) can write SetAlive().
   Writes block ALL reads and other writes.
```

---

## 6. STRUCTS AND VARIABLE TRACE
For each major phase, these are the key data structures involved:

### 1. Request Representation
* **Struct**: `http.Request` (std-library)
* **Fields**:
  * `Method`: `"GET"`
  * `URL`: `/api/users`
  * `RemoteAddr`: `"192.168.1.50:52410"` (client's IP/port)
  * `Header`: Map containing HTTP headers.

### 2. Backend Representation
* **Struct**: `loadbalancer.Backend`
* **Fields**:
  * `URL`: `*url.URL` representing `http://127.0.0.1:8081`.
  * `Alive`: Boolean representing health status (guarded by `mu`).
  * `ReverseProxy`: `*httputil.ReverseProxy` instance.
  * `mu`: `sync.RWMutex` protecting `Alive`.

### 3. Selection State
* **Struct**: `loadbalancer.BackendPool`
* **Fields**:
  * `backends`: Slice of `*Backend` pointers.
  * `current`: `uint64` counter (accessed via `sync/atomic`).

---

## 7. CODE PATH TRACE — PACKAGE-BY-PACKAGE CALL FLOW

```
┌───────────┐      ┌────────────┐      ┌──────────┐      ┌────────────────┐
│  main.go  │ ───► │ server.go  │ ───► │ proxy.go │ ───► │ roundrobin.go  │
│           │      │            │      │          │      │                │
│ Bootstrap │      │ Listener   │      │ ServeHTTP│      │ NextBackend()  │
│ + Config  │      │ + Shutdown │      │ + Fwd    │      │ + Atomic Index │
└───────────┘      └────────────┘      └──────────┘      └────────────────┘
     │                                                          │
     │              ┌────────────┐                              │
     └─────────────►│ checker.go │◄─────────────────────────────┘
                    │            │     (reads/writes same Backend structs)
                    │ Health     │
                    │ Polling    │
                    └────────────┘
```

### Step 1: Entry Point (`main.go`)
Bootstraps config, backends in background goroutines, connects proxies, and launches `server.Start()`.

### Step 2: Connection Accept (`server/server.go`)
* `gs.httpServer.ListenAndServe()` binds to `:8080`.
* The TCP listener calls `Accept()` in a loop.
* Once a connection is accepted, `net/http` launches a new goroutine (e.g. `goroutine-42`) to run `c.serve()`.

### Step 3: Routing Execution (`proxy/proxy.go`)
* `goroutine-42` executes `GatewayHandler.ServeHTTP(w, r)`.
* It calls `gh.Pool.NextBackend()`.

### Step 4: Backend Selection (`loadbalancer/roundrobin.go`)
* `NextBackend()` executes.
* It increments `bp.current` atomically.
* For each candidate, it checks `candidate.IsAlive()`, which calls `RLock()` and `RUnlock()` on `candidate.mu`.
* It returns the chosen `Backend` struct pointer.

### Step 5: Forwarding (`proxy/proxy.go` / `httputil.ReverseProxy`)
* `ServeHTTP` delegates to `selectedBackend.ReverseProxy.ServeHTTP(w, r)`.
* The `Director` executes, modifying the cloned request.
* `RoundTrip` acquires/creates a connection to the backend and writes HTTP data.
* If a write error occurs, the `ErrorHandler` function is triggered. It runs the failure callback:
  ```go
  func(err error) {
      b.SetAlive(false) // b is captured in the closure
  }
  ```
  `SetAlive()` acquires `b.mu.Lock()`, sets `Alive = false`, and releases `b.mu.Unlock()`.
* If successful, the response body is streamed to the client's socket.

---

## 8. GRACEFUL SHUTDOWN SEQUENCE

```
                  Normal Operation
                        │
          ┌─────────────▼─────────────┐
          │   User presses Ctrl+C     │
          │   (SIGINT received)       │
          └─────────────┬─────────────┘
                        │
          ┌─────────────▼─────────────┐
          │  ① checker.Stop()         │
          │     close(stopChan)       │
          │     wg.Wait()             │    ← Health checker goroutine exits
          └─────────────┬─────────────┘
                        │
          ┌─────────────▼─────────────┐
          │  ② httpServer.Shutdown()  │
          │     Close listener socket │    ← No new connections accepted
          │     Close idle keep-alives│    ← Free unused sockets
          │     Wait for active reqs  │    ← G1, G2... finish their work
          └─────────────┬─────────────┘
                        │
               Timeout? ├────── YES ──► httpServer.Close()  (force kill)
                        │
                       NO
                        │
          ┌─────────────▼─────────────┐
          │  ③ All goroutines done    │
          │     Process exits cleanly │
          └───────────────────────────┘
```

---

## 9. INTERVIEW QUESTIONS
1. **Q**: Walk me through the exact path a request takes from the client's network card to a backend server through your gateway.
   * **A**: The client sends packets triggering a TCP handshake accepted by our listener socket. The Go runtime maps this connection to a new goroutine running the `net/http` server loop. The server parses HTTP headers into `http.Request`. Our `GatewayHandler.ServeHTTP` intercepts it, calls `Pool.NextBackend()` which atomically increments the selection counter and checks candidate health under a read-lock, retrieves a backend, and calls `ReverseProxy.ServeHTTP`. This runs our Director to rewrite headers, routes the request via an idle socket in the connection pool, reads the response, and streams it back to the client.
2. **Q**: Which goroutine executes the load-balancing search, and which goroutine executes the health check?
   * **A**: The load-balancing search runs on the **client request goroutine** spawned dynamically by the `net/http` server for that connection. The health check runs on a **background scheduler goroutine** spawned once during gateway startup, which then spawns nested short-lived goroutines to check downstream backends in parallel.
3. **Q**: What happens to variables locked in a closure when a callback is registered?
   * **A**: Go supports closures, meaning nested functions can reference variables defined in their outer scope. When we register the failure callback, it closes over the `*loadbalancer.Backend` pointer. Even after the bootstrap function exits, the callback retains the memory address of the backend, allowing it to modify its health state directly when a routing failure occurs.
