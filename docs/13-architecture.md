# 13. System Architecture and Design

## 1. CONCEPT
A well-designed software project enforces clean separation of concerns, ensuring each package owns a single, well-defined responsibility. This makes the system understandable, testable, and maintainable.

---

## 2. WHY IT EXISTS
If we write the gateway, health checker, proxy, and load balancer in a single file or package:
* Code becomes tightly coupled and difficult to navigate.
* Testing components in isolation becomes impossible.
* Implementing new features (like a new load-balancing algorithm) requires modifying unrelated code (like the HTTP server or header rewrites), violating the **Open-Closed Principle** (SOLID).

---

## 3. FULL SYSTEM ARCHITECTURE

```
╔══════════════════════════════════════════════════════════════════════════════════╗
║                          SYSTEM ARCHITECTURE                                  ║
╠══════════════════════════════════════════════════════════════════════════════════╣
║                                                                                ║
║                           ┌──────────────┐                                     ║
║                           │    CLIENT    │                                     ║
║                           │  (Browser /  │                                     ║
║                           │   curl /app) │                                     ║
║                           └──────┬───────┘                                     ║
║                                  │                                             ║
║                                  │ HTTP Request                                ║
║                                  ▼                                             ║
║   ╔══════════════════════════════════════════════════════════════════════╗      ║
║   ║                      GATEWAY  (:8080)                              ║      ║
║   ║                                                                     ║      ║
║   ║   ┌─────────────┐    ┌──────────────────┐    ┌─────────────────┐   ║      ║
║   ║   │   server/   │    │     proxy/       │    │  loadbalancer/  │   ║      ║
║   ║   │             │    │                  │    │                 │   ║      ║
║   ║   │ • Listen    │───►│ • GatewayHandler │───►│ • BackendPool   │   ║      ║
║   ║   │ • Accept    │    │ • ReverseProxy   │    │ • NextBackend() │   ║      ║
║   ║   │ • Shutdown  │    │ • Headers        │    │ • Round Robin   │   ║      ║
║   ║   │ • Signals   │    │ • ErrorHandler   │    │ • Atomic Index  │   ║      ║
║   ║   └─────────────┘    └──────────────────┘    └────────┬────────┘   ║      ║
║   ║          │                                            │            ║      ║
║   ║          │            ┌──────────────────┐            │            ║      ║
║   ║          └───────────►│     health/      │◄───────────┘            ║      ║
║   ║                       │                  │                         ║      ║
║   ║                       │ • Ticker Loop    │  (reads & writes        ║      ║
║   ║                       │ • Parallel Pings │   Backend.Alive)        ║      ║
║   ║                       │ • State Toggle   │                         ║      ║
║   ║                       └──────────────────┘                         ║      ║
║   ║                                                                     ║      ║
║   ╚════════════════════════════╪═════════════════════════════════════════╝      ║
║                                │                                               ║
║                    ┌───────────┴───────────┐                                   ║
║                    │                       │                                   ║
║                    ▼                       ▼                                   ║
║   ╔═════════════════════════╗   ╔═════════════════════════╗                    ║
║   ║    BACKEND A (:8081)    ║   ║    BACKEND B (:8082)    ║                    ║
║   ║                         ║   ║                         ║                    ║
║   ║   GET /       → Reply   ║   ║   GET /       → Reply   ║                    ║
║   ║   GET /health → OK/FAIL ║   ║   GET /health → OK/FAIL ║                    ║
║   ║   GET /toggle → Flip    ║   ║   GET /toggle → Flip    ║                    ║
║   ║                         ║   ║                         ║                    ║
║   ╚═════════════════════════╝   ╚═════════════════════════╝                    ║
║                                                                                ║
╚══════════════════════════════════════════════════════════════════════════════════╝
```

---

## 4. PACKAGE DEPENDENCY GRAPH

This shows **which package imports which**. Arrows point from importer → imported.
The key design rule: `loadbalancer` is a **leaf package** — nothing in the project imports it backwards.

```
                           ┌──────────────────────────────────────┐
                           │             main.go                  │
                           │         (Orchestrator)               │
                           └──┬────┬────┬────┬────┬────┬─────────┘
                              │    │    │    │    │    │
              ┌───────────────┘    │    │    │    │    └───────────────┐
              ▼                    ▼    │    ▼    │                    ▼
     ┌──────────────┐    ┌─────────┐   │  ┌──────┴─────┐    ┌──────────────┐
     │   config/    │    │ backend/│   │  │   proxy/   │    │   server/    │
     │              │    │         │   │  │            │    │              │
     │ • LoadConfig │    │ • Sim   │   │  │ • NewProxy │    │ • Start()   │
     │ • Flags      │    │ • Toggle│   │  │ • Gateway  │    │ • Shutdown  │
     └──────────────┘    └─────────┘   │  │   Handler  │    └──────┬───────┘
                                       │  └──────┬─────┘           │
                              ┌────────┘         │                 │
                              ▼                  ▼                 ▼
                     ┌──────────────┐   ┌──────────────────────────────┐
                     │   health/    │   │       loadbalancer/          │
                     │              │   │                              │
                     │ • Checker    │──►│ • Backend struct             │
                     │ • Parallel   │   │ • BackendPool struct         │
                     │   Pings      │   │ • NextBackend() (atomic)     │
                     └──────────────┘   │ • SetAlive() / IsAlive()     │
                                        │                              │
                                        │   *** LEAF PACKAGE ***       │
                                        │   Imports NOTHING from       │
                                        │   this project               │
                                        └──────────────────────────────┘


   DEPENDENCY RULES:
   ┌──────────────────────────────────────────────────────────────────┐
   │  loadbalancer  imports:  NOTHING (pure data structures)         │
   │  proxy         imports:  loadbalancer                           │
   │  health        imports:  loadbalancer                           │
   │  server        imports:  health  (to stop checker on shutdown)  │
   │  main.go       imports:  ALL packages (wires everything)        │
   │  config        imports:  NOTHING (only std lib)                 │
   │  backend       imports:  NOTHING (only std lib)                 │
   └──────────────────────────────────────────────────────────────────┘

   WHY THIS MATTERS:
   • No circular dependencies (Go compiler would reject them)
   • loadbalancer can be tested with ZERO other packages
   • Adding "Least Connections" only touches loadbalancer/
   • Adding "rate limiting" only touches proxy/
```

---

## 5. PACKAGE FILE MAP — WHAT LIVES WHERE

```
┌──────────────────────────────────────────────────────────────────────┐
│  GOLANGSERVER/                                                       │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  main.go                                                     │    │
│  │  ─────────────────────────────────────────────────           │    │
│  │  ROLE: Orchestrator. Creates everything, connects them,      │    │
│  │        starts backends in goroutines, calls server.Start().  │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌───────────────────┐  ┌───────────────────┐  ┌──────────────────┐ │
│  │  config/          │  │  backend/         │  │  server/         │ │
│  │  config.go        │  │  backend.go       │  │  server.go       │ │
│  │  ───────────      │  │  ───────────      │  │  ───────────     │ │
│  │  CLI flags,       │  │  Simulated HTTP   │  │  HTTP listener,  │ │
│  │  addresses,       │  │  servers with     │  │  OS signal trap, │ │
│  │  intervals,       │  │  /health and      │  │  graceful        │ │
│  │  timeouts         │  │  /toggle          │  │  shutdown        │ │
│  └───────────────────┘  └───────────────────┘  └──────────────────┘ │
│                                                                      │
│  ┌───────────────────┐  ┌───────────────────┐  ┌──────────────────┐ │
│  │  loadbalancer/    │  │  proxy/           │  │  health/         │ │
│  │  roundrobin.go    │  │  proxy.go         │  │  checker.go      │ │
│  │  ───────────      │  │  ───────────      │  │  ───────────     │ │
│  │  Backend struct,  │  │  ReverseProxy     │  │  Background      │ │
│  │  BackendPool,     │  │  wrapper, header  │  │  ticker loop,    │ │
│  │  atomic counter,  │  │  rewriting,       │  │  parallel pings, │ │
│  │  RWMutex guards   │  │  GatewayHandler   │  │  state updates   │ │
│  └───────────────────┘  └───────────────────┘  └──────────────────┘ │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │  tests/                                                      │    │
│  │  ─────────────────────────────────────────────────           │    │
│  │  roundrobin_test.go  — selection + concurrency tests         │    │
│  │  proxy_test.go       — header rewrite + failure tests        │    │
│  │  integration_test.go — full flow: routing, failover, recover │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 6. HEALTH STATE MACHINE

```
                    ┌──────────────────────────────┐
                    │                              │
                    ▼                              │
         ╔══════════════════╗             ╔══════════════════╗
         ║                  ║  Health     ║                  ║
         ║     ONLINE       ║  check     ║    OFFLINE       ║
         ║   (Receiving     ║  fails     ║  (Excluded from  ║
         ║    traffic)      ║ ─────────► ║   routing pool)  ║
         ║                  ║    OR      ║                  ║
         ╚══════════════════╝  Proxy     ╚══════════════════╝
                    ▲           error              │
                    │          handler             │
                    │                              │
                    │    Health check              │
                    │    returns 200 OK            │
                    └──────────────────────────────┘

   TRIGGERS FOR GOING OFFLINE:
   1. Active health check: GET /health returns non-200 or times out
   2. Passive detection:   proxy.ErrorHandler fires on connection failure

   TRIGGER FOR GOING ONLINE:
   1. Active health check: GET /health returns 200 OK
      (Passive detection CANNOT bring a backend back online —
       it needs the active poller to confirm recovery)
```

---

## 7. INTERNALS: DECOUPLING VIA INTERFACES
In production systems, load-balancing algorithms are abstracted behind interfaces. This allows switching strategies at runtime:

```go
package loadbalancer

// Balancer defines the interface for selection algorithms.
type Balancer interface {
	Next() *Backend
}
```

By coding `GatewayHandler` to rely on a `Balancer` interface rather than a concrete `BackendPool` struct, we decouple the proxy from selection algorithms completely.

---

## 8. TRADEOFFS
### Split-package Architecture vs. Single-Package Layout
* **Split Packages (Our Choice)**:
  * *Pros*: High decoupling; clear file responsibilities; easy unit testing; prevents circular dependencies; matches professional architectures.
  * *Cons*: Requires importing multiple packages; circular dependency imports (Go compiler error) can occur if not designed carefully.
* **Single Package (Flat structure)**:
  * *Pros*: Simple imports; no circular dependency errors.
  * *Cons*: Messy namespace; easy to write tightly coupled code; hard for a reviewer to identify boundaries.

---

## 9. INTERVIEW QUESTIONS
1. **Q**: Explain the dependency layout of your load balancer. Why did you separate them this way?
   * **A**: I structured the codebase into decoupled packages (`config`, `backend`, `loadbalancer`, `proxy`, `health`, `server`). The goal is to enforce the single responsibility principle. `loadbalancer` owns the state and selection logic. `proxy` owns header modification and HTTP forwarding. `health` owns polling cycles. Separating them this way ensures changes to one (like adding a Least Connections algorithm) do not affect others (like HTTP socket draining).
2. **Q**: What is a circular dependency in Go, and how does your architecture avoid it?
   * **A**: A circular dependency occurs when package A imports package B, and package B directly or indirectly imports package A. The Go compiler rejects this. We avoid it by keeping downstream packages (like `loadbalancer`) completely leaf-level (they do not import any other project packages). High-level orchestrators (like `main.go` or `server.go`) tie them together.
3. **Q**: How would you modify your architecture to support a new load-balancing strategy, like Least Connections?
   * **A**: I would define a `Balancer` interface in the `loadbalancer` package containing a `NextBackend()` method. I would implement this interface on both `RoundRobinBalancer` and `LeastConnectionsBalancer` structs. Then, I would update the `GatewayHandler` to accept the `Balancer` interface rather than a concrete pool struct. This allows switching algorithms dynamically without changing the proxy routing code.
