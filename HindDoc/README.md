# 🚀 GoGateway - HindDoc (Hinglish Learning Guides)

Welcome to **HindDoc**! Agar aapne C++ me OOPs, Pointers, Threads, ya Mutex thoda padha tha aur ab bhuul gaye ho, aur aap samajhna chahte ho ki **Go (Golang)** me ye sab kaise kaam karta hai aur humne iss **GoGateway (Load Balancer & Reverse Proxy)** project me inko kaise implement kiya hai — toh ye docs bilkul aapke liye bane hain!

---

## 📚 Complete Learning Roadmap

| Doc File | Primary Topic | C++ Connection & Core Focus |
| :--- | :--- | :--- |
| **[01-go-vs-cpp-core-concepts.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/01-go-vs-cpp-core-concepts.md)** | **C++ vs Go Bridge** | `std::thread` vs **Goroutines** (GMP Model), `std::mutex` vs `sync.RWMutex`, C++ Class vs Go **Struct & Methods**, Pointers & GC. |
| **[02-goroutines-and-concurrency-in-code.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/02-goroutines-and-concurrency-in-code.md)** | **Goroutines, Ticker & Channels** | `main.go` backend spawning, `health/checker.go` ticker loop, `sync.WaitGroup`, aur `chan struct{}` signal control. |
| **[03-mutex-race-condition-and-memory-safety.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/03-mutex-race-condition-and-memory-safety.md)** | **Race Conditions & Mutexes** | `loadbalancer/roundrobin.go` me `sync.RWMutex` (Read/Write Locks) aur `atomic.AddUint64` for lock-free round-robin. |
| **[04-structs-handlers-and-closures-explained.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/04-structs-handlers-and-closures-explained.md)** | **Handlers, Proxies & Closures** | `http.Handler` interface, `GatewayHandler` struct, `httputil.ReverseProxy`, aur `main.go` me passive failure closure trick. |
| **[05-full-code-walkthrough-and-live-flow.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/05-full-code-walkthrough-and-live-flow.md)** | **Live Request Flow & File Map** | Client request se backend tak har step ka live execution flow, struct-by-struct mapping aur failure recovery. |
| **[06-system-design-load-balancer-deep-dive.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/06-system-design-load-balancer-deep-dive.md)** | **System Design: Load Balancers** | L4 vs L7, Socket Level, DSR (Direct Server Return), Consistent Hashing (Hash Ring + Virtual Nodes), Active vs Passive Health Checks. |
| **[07-system-design-rate-limiter-deep-dive.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/07-system-design-rate-limiter-deep-dive.md)** | **System Design: Rate Limiting** *(Note: Not implemented in code)* | 5 Core Algorithms (Token Bucket, Leaky Bucket, Sliding Window Counter), Redis Lua Script Atomicity, Race Conditions, HTTP 429 & Headers. |
| **[08-ti-interview-cn-os-100-qa-master-guide.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/08-ti-interview-cn-os-100-qa-master-guide.md)** | **TI / Core Systems 100 Q&A** | 100 Questions & Answers spanning Computer Networks, OS Internals, TCP/IP, Sockets, Epoll, Concurrency, and System Design. |

---

## 🏛️ High-Level System Architecture

Ye Gateway client traffic ko receive karta hai, round-robin algorithm ke through active backend choose karta hai, request proxy karta hai, aur background me backends ki health monitoring karta hai.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          EXTERNAL CLIENT LAYER                          │
│         📱 App / 💻 Browser  ──►  HTTP Request GET /api/data            │
└────────────────────────────────────┬────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                    🛡️ GOGATEWAY SYSTEM (Port :8080)                     │
│  ┌───────────────────────────┐         ┌─────────────────────────────┐  │
│  │       GatewayServer       │ ──────► │       GatewayHandler        │  │
│  │     (server/server.go)    │         │      (proxy/proxy.go)       │  │
│  └───────────────────────────┘         └──────────────┬──────────────┘  │
│                                                       │                 │
│  ┌───────────────────────────┐                        ▼                 │
│  │   HealthChecker Ticker    │ ──────► ┌─────────────────────────────┐  │
│  │    (health/checker.go)    │ Update  │  BackendPool (Atomic Next)  │  │
│  └─────────────┬─────────────┘ Status  │ (loadbalancer/roundrobin.go)│  │
│                │                       └─────────────────────────────┘  │
└────────────────┼────────────────────────────────────────────────────────┘
                 │ Active Health                               │ Proxy
                 │ Checks GET /health                          │ Traffic
                 ▼                                             ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       DOWNSTREAM BACKEND CLUSTER                        │
│   ┌─────────────────────┐   ┌─────────────────────┐   ┌──────────────┐  │
│   │   Backend 1 (:8081) │   │   Backend 2 (:8082) │   │Backend3:8083 │  │
│   │ (SimulatedBackend)  │   │ (SimulatedBackend)  │   │ (DOWN / 500) │  │
│   └─────────────────────┘   └─────────────────────┘   └──────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 🛠️ How to Run & Verify

1. **Start Gateway & Backends**:
   ```bash
   go run main.go
   ```
2. **Send Test HTTP Requests**:
   ```bash
   curl http://localhost:8080/
   ```
3. **Simulate Backend Failure (Manual Toggle)**:
   ```bash
   curl http://localhost:8081/toggle
   ```
   Watch the Health Checker detect the failure within 2 seconds and route traffic away automatically!

---
*Docs generated specifically for GoGateway codebase comprehension.*
