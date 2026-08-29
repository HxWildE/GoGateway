# GoGateway: Concurrent HTTP Reverse Proxy & Load Balancer in Go

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%201.26+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version" />
  <img src="https://img.shields.io/badge/Dependencies-0%20(Stdlib%20Only)-success?style=for-the-badge" alt="Zero Dependencies" />
  <img src="https://img.shields.io/badge/Concurrency-Lock--Free%20Hotpath-blueviolet?style=for-the-badge" alt="Lock Free" />
  <img src="https://img.shields.io/badge/Tests-6%2F6%20Passed%20(Race--Safe)-brightgreen?style=for-the-badge" alt="Tests Passed" />
</p>

A production-grade, lightweight, and high-performance Layer-7 Reverse Proxy and Load Balancer engineered strictly with **Go's Standard Library** (zero third-party dependencies). 

Designed to demonstrate core **systems engineering**, **computer networking**, **lock-free concurrency**, and **failure resilience** concepts for scalable distributed backend architectures.

---

## 1. Key Engineering Highlights

* **Lock-Free Request Path**: Uses atomic 64-bit integer fetch-and-add operations (`sync/atomic`) for Round-Robin selection, eliminating mutex contention on high-throughput request paths.
* **Dual-Mode Failure Detection**:
  * **Active Monitoring**: Background ticker polls health endpoints across downstream servers in parallel via concurrent goroutines.
  * **Passive Detection**: Closure-based `ErrorHandler` traps routing failures during live request forwarding to instantly evict dead nodes with zero delay.
* **Granular Concurrency & Memory Safety**: Health states are protected using Read-Write Mutexes (`sync.RWMutex`), enabling hundreds of concurrent reader goroutines while strictly serializing status transitions.
* **Clean Network Layering (Two Network Legs)**: Transparently isolates backend topologies from public clients, modifying headers (`X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Forwarded-For`) and enforcing connection pooling timeouts.
* **Graceful Termination & Drain**: Traps OS termination signals (`SIGINT`, `SIGTERM`), halts the listener socket, terminates background tickers, and drains active HTTP connections within a bounded context timeout.

---

## 2. System Architecture

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
║                                  │ HTTP Request (LEG 1)                        ║
║                                  ▼                                             ║
║   ╔══════════════════════════════════════════════════════════════════════╗      ║
║   ║                 GOGATEWAY PROXY SERVER (:8080)                       ║      ║
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
║                                │ HTTP Request Forwarding (LEG 2)               ║
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

## 3. Package Structure & Modular Design

Each package owns a single, well-isolated responsibility with strict unidirectional dependencies:

```
GoGateway/
├── main.go                     # Orchestrator & bootstrap wiring
├── config/
│   └── config.go               # Command-line configuration parsing
├── backend/
│   └── backend.go              # Mock target HTTP servers with dynamic /toggle
├── loadbalancer/
│   └── roundrobin.go           # Thread-safe backend pool & atomic round-robin (Leaf Package)
├── proxy/
│   └── proxy.go                # Custom reverse proxy handler & header rewriter
├── health/
│   └── checker.go              # Active concurrent background health polling engine
├── server/
│   └── server.go               # TCP listener socket lifecycle & graceful shutdown
└── tests/
    ├── roundrobin_test.go      # Selection algorithms & concurrency race tests
    ├── proxy_test.go           # Header rewrite & passive failure callback tests
    └── integration_test.go     # End-to-end load balancing, failover & recovery
```

---

## 4. Quick Start & Execution

### 1. Run the Gateway Server
The application automatically spins up two mock backend servers at `127.0.0.1:8081` and `127.0.0.1:8082` alongside the gateway listener at `:8080`:

```bash
go run main.go
```

#### Custom CLI Flags:
```bash
go run main.go \
  -gateway=:8080 \
  -backends=127.0.0.1:8081,127.0.0.1:8082 \
  -health-interval=5s \
  -health-timeout=2s \
  -proxy-timeout=10s
```

---

## 5. Live Failure & Failover Demonstration

### 1. Test Load Balancing
Send successive HTTP requests to the gateway on `:8080`:
```bash
curl http://localhost:8080/
curl http://localhost:8080/
```
**Response (Alternating Distribution):**
```text
Hello from backend server running at 127.0.0.1:8081! Request Path: /
Hello from backend server running at 127.0.0.1:8082! Request Path: /
```

---

### 2. Simulate Node Failure
Trigger simulated failure on Backend A via its `/toggle` endpoint:
```bash
curl http://localhost:8081/toggle
```
```text
Backend 127.0.0.1:8081 status toggled to UNHEALTHY
```

Now, query the gateway again:
```bash
curl http://localhost:8080/
curl http://localhost:8080/
```
**Response (Automatic Failover):**
```text
Hello from backend server running at 127.0.0.1:8082! Request Path: /
Hello from backend server running at 127.0.0.1:8082! Request Path: /
```

---

### 3. Verify Node Recovery
Restore Backend A back to health:
```bash
curl http://localhost:8081/toggle
```
Within **5 seconds**, the background health checker detects recovery, and traffic seamlessly distributes across both servers again.

---

## 6. Automated Testing & Verification

Run the entire suite of unit and integration tests:

```bash
go test -v ./tests/
```

### Run with Race Detector (Zero Data Races):
```bash
go test -race -v ./tests/
```

**Test Suite Coverage:**
* `TestRoundRobinSelection`: Validates sequential circular request distribution.
* `TestUnhealthyBackendExclusion`: Verifies instant exclusion of downed instances.
* `TestConcurrentBackendSelection`: Runs **50 concurrent goroutines** making 1000 requests each with zero race conditions.
* `TestProxyForwardingAndHeaders`: Validates header sanitization (`X-Forwarded-*`).
* `TestProxyPassiveFailureTrigger`: Validates 502 status response and immediate failure callbacks.
* `TestGatewayIntegration`: Validates full end-to-end failover and recovery flow.

---

## 7. Systems & Networking Design Concepts

| Mechanism | Implementation | Benefit |
|---|---|---|
| **Round Robin Hotpath** | `atomic.AddUint64(&bp.current, 1)` | Zero lock contention, predictable $O(1)$ latency. |
| **Health State Locking** | `sync.RWMutex` (RLock / Lock) | Allows hundreds of concurrent reads; exclusive writes only during state change. |
| **Connection Pooling** | `http.Transport` with custom `Dialer` | Socket reuse via keep-alive, bounds file descriptors, prevents socket exhaustion. |
| **Graceful Shutdown** | `http.Server.Shutdown(ctx)` | In-flight requests drain safely; no connections abruptly severed. |
| **Zero Allocations Routing** | Fixed slice index search with modulo | Low memory overhead, eliminates garbage collection spikes. |

---

## 8. Technical Interview Preparation Roadmap

The repository includes deep-dive engineering modules under `docs/` covering core systems, networking, and Go internals:

* [01. Go Basics Needed for Systems](docs/01-go-basics-needed.md)
* [02. Go HTTP Server Architecture](docs/02-http-server.md)
* [03. TCP and Networking Fundamentals](docs/03-tcp-and-networking.md)
* [04. Reverse Proxy Mechanics and httputil](docs/04-reverse-proxy.md)
* [05. Load Balancing Design Patterns](docs/05-load-balancing.md)
* [06. Round-Robin & Atomic Operations](docs/06-round-robin.md)
* [07. Go Concurrency & Schedulers](docs/07-concurrency.md)
* [08. Race Conditions & Mutexes](docs/08-race-conditions-and-mutex.md)
* [09. Active & Passive Health Checking](docs/09-health-checking.md)
* [10. Failure Handling & Edge Cases](docs/10-failure-handling.md)
* [11. Graceful Shutdown Design](docs/11-graceful-shutdown.md)
* [12. Request Runtime Flow Trace](docs/12-runtime-flow.md)
* [13. System Architecture & Design](docs/13-architecture.md)
* [14. Testing & Benchmarking Services](docs/14-testing-and-benchmarking.md)
* [15. Interview Preparation & TI Question Bank](docs/15-interview-preparation.md)

---

## 9. Resume Bullets (Systems & Backend Focus)

* *Designed and developed a high-concurrency Layer-7 Reverse Proxy and Load Balancer in Go with zero external dependencies, leveraging atomic integer operations (`sync/atomic`) for lock-free request routing.*
* *Implemented dual-layer health monitoring using concurrent active background polling (`time.Ticker` + `sync.WaitGroup`) and closure-based passive failure interception to achieve instantaneous zero-downtime failover.*
* *Engineered transport connection pooling and socket timeout controls, integrating OS signal handlers for graceful shutdown to safely drain active TCP sessions during process restarts.*
