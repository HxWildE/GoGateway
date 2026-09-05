# 01. End-to-End System Architecture (HLD & LLD)

```
========================================================================================
                                 COMPLETE SYSTEM BLUEPRINT
========================================================================================

                 [ 📱 Mobile App / 💻 Web Browser (Client) ]
                                    │
                                    │  1. HTTP GET /api/data (Port :8080)
                                    ▼
       ┌─────────────────────────────────────────────────────────────┐
       │             🛡️ GOGATEWAY PROCESS (:8080)                   │
       │                                                             │
       │   ┌─────────────────────────────────────────────────────┐   │
       │   │  server.GatewayServer (Orchestrator Lifecycle)      │   │
       │   │  - ListenAndServe(":8080")                          │   │
       │   │  - Graceful Shutdown Channel (os.Interrupt)         │   │
       │   └──────────────────────────┬──────────────────────────┘   │
       │                              │                              │
       │                              ▼                              │
       │   ┌─────────────────────────────────────────────────────┐   │
       │   │  proxy.GatewayHandler (HTTP Entrypoint)             │   │
       │   │  - ServeHTTP(w, r)                                  │   │
       │   └──────────────────────────┬──────────────────────────┘   │
       │                              │                              │
       │             2. pool.GetNextBackend()                        │
       │                              │                              │
       │   ┌──────────────────────────▼──────────────────────────┐   │
       │   │  loadbalancer.BackendPool                           │   │
       │   │  - sync.RWMutex (Protects slice of *Backend)        │   │
       │   │  - atomic.AddUint64(&cursor, 1) (Lock-free modulo)  │   │
       │   └───────────────┬─────────────────────┬───────────────┘   │
       │                   │                     │                   │
       │    3. Selected    │                     │ Health Updates    │
       │       *Backend    ▼                     ▲ (Active)          │
       │   ┌──────────────────────────┐   ┌──────┴───────────────┐   │
       │   │ httputil.ReverseProxy    │   │ health.HealthChecker │   │
       │   │ - Director (Rewrites URL)│   │ - time.Ticker (5s)   │   │
       │   │ - ErrorHandler (Closure) │   │ - Probe /healthz     │   │
       │   └───────────────┬──────────┘   └──────────────────────┘   │
       │                   │                                         │
       └───────────────────┼─────────────────────────────────────────┘
                           │ 4. Proxied TCP Connection
                           │
             ┌─────────────┼─────────────┐
             ▼             ▼             ▼
      ┌─────────────┐┌─────────────┐┌─────────────┐
      │  Backend 1  ││  Backend 2  ││  Backend 3  │
      │   (:8081)   ││   (:8082)   ││   (:8083)   │
      │  [HEALTHY]  ││  [HEALTHY]  ││   [DEAD]    │
      └─────────────┘└─────────────┘└─────────────┘
```

---

## 🔄 Live Interactive Flow (Step-by-Step)

```mermaid
sequenceDiagram
    autonumber
    actor Client as 💻 Client Browser
    participant Srv as 🛡️ GatewayServer (:8080)
    participant Handler as ⚙️ GatewayHandler
    participant Pool as ⚖️ BackendPool
    participant Proxy as 🔄 ReverseProxy
    participant Backend as 🖥️ Backend 1 (:8081)
    participant Health as 🩺 HealthChecker

    Note over Health,Backend: Background Active Monitoring (Every 5s)
    Health->>Backend: GET /healthz (Active Ping)
    Backend-->>Health: HTTP 200 OK (Alive = true)

    Note over Client,Backend: Live User Request Flow
    Client->>Srv: HTTP GET /user/profile
    Srv->>Handler: Route to ServeHTTP(w, r)
    Handler->>Pool: GetNextBackend()
    Note over Pool: atomic.AddUint64(&cursor, 1)<br/>Filter Alive == true
    Pool-->>Handler: Return *Backend (Backend 1)
    Handler->>Proxy: ReverseProxy.ServeHTTP(w, r)
    Note over Proxy: Director modifies req.URL to :8081
    Proxy->>Backend: Forwarded HTTP Request
    Backend-->>Proxy: HTTP 200 OK + JSON Body
    Proxy-->>Client: Final Response Streamed Back
```

---

## 🧩 Struct and Component Mapping

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. config.Config        : Holds CLI flags (GatewayAddr, Backends, Timeouts) │
│ 2. backend.Simulated    : Runs mock HTTP servers on :8081, :8082, :8083     │
│ 3. loadbalancer.Backend : Holds URL, Alive (bool), sync.RWMutex, ReverseProxy│
│ 4. loadbalancer.Pool    : Holds []*Backend, cursor (uint64), sync.RWMutex   │
│ 5. proxy.NewProxy       : Wraps httputil.ReverseProxy + ErrorHandler closure│
│ 6. health.HealthChecker : Ticker loop checking /healthz every N seconds     │
│ 7. server.GatewayServer : Manages http.Server & Graceful OS signal trap     │
└─────────────────────────────────────────────────────────────────────────────┘
```
