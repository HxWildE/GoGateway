# 05. Live Execution Flow & Full Code Walkthrough 🚀

Is final chapter me hum poore **GoGateway** system ka **Live Execution Flow** tracing ke saath dekhenge — System startup se lekar live client request routing, backend failure detection, aur Graceful Shutdown tak!

---

## 🗺️ Code Map & Responsibilities

| File Path | Core Struct / Function | Primary Responsibility |
| :--- | :--- | :--- |
| **[config/config.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/config/config.go)** | `Config`, `LoadConfig()` | Command-line flags (`-backends`, `-port`, `-health-interval`) parse karna. |
| **[backend/backend.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/backend/backend.go)** | `SimulatedBackend` | Mock downstream HTTP servers (:8081, :8082, :8083) run karna aur `/toggle` endpoint handle karna. |
| **[loadbalancer/roundrobin.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/loadbalancer/roundrobin.go)** | `BackendPool`, `Backend`, `NextBackend()` | Atomic Round-Robin selection aur `sync.RWMutex` safe `Alive` state management. |
| **[proxy/proxy.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/proxy/proxy.go)** | `GatewayHandler`, `NewProxy()` | Client request ko downstream backend par HTTP reverse proxying karna aur passive closure trigger karna. |
| **[health/checker.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/health/checker.go)** | `HealthChecker` | Background ticker loop (every 2s) jo saare backends par `GET /health` active polling karti hai. |
| **[server/server.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/server/server.go)** | `GatewayServer`, `Start()` | Gateway Server lifecycle, OS signal listener (SIGINT/SIGTERM), aur Graceful Shutdown drain. |
| **[main.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/main.go)** | `main()` | Orchestrator jo sabhi modules ko wire-up karke system launch karta hai. |

---

## 🎬 Step-by-Step System Lifecycle

### Step 1: System Boot & Configuration (`main.go` -> `config.go`)
1. User Terminal me command chalata hai: `go run main.go`.
2. [config/config.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/config/config.go#L20-L40) command line flags parse karta hai aur backend addresses (`127.0.0.1:8081`, `127.0.0.1:8082`, `127.0.0.1:8083`) load karta hai.

### Step 2: Launching Simulated Backends (`main.go` -> `backend.go`)
[main.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/main.go#L24-L36) loop me har address ke liye `backend.NewSimulatedBackend(addr)` banata hai aur unko **3 background goroutines** me `go srv.Start()` se listener launch karta hai.

### Step 3: Wiring Backend Pool & Passive Closures (`main.go` -> `loadbalancer` & `proxy`)
1. `pool := loadbalancer.NewBackendPool()` instantiate hota hai.
2. Har backend address ke liye `proxy.NewProxy` function me **passive failure callback closure** create karke `pool.AddBackend(b)` kiya jata hai.

### Step 4: Starting Health Checker & Gateway Server (`server.go` & `checker.go`)
1. [health/checker.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/health/checker.go#L39-L67) launch hota hai aur `time.NewTicker(2 * time.Second)` background loop me chalne lagta hai.
2. [server/server.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/server/server.go#L36-L50) port `:8080` par HTTP Listener start karta hai.

---

## 🌊 Live Request Execution Sequence

Jab koi Client `curl http://localhost:8080/api/v1/users` call karta hai:

```
[CLIENT] ────────► HTTP GET /api/v1/users on :8080
                       │
                       ▼
             [GatewayServer (server.go)]
                       │  Spawns Goroutine & calls ServeHTTP
                       ▼
            [GatewayHandler (proxy.go)]
                       │  Calls NextBackend()
                       ▼
           [BackendPool (roundrobin.go)]
                       │  Atomic counter increment & RLock() check
                       ▼
             [Selected Backend (*Backend)]
                       │  Passes to targetBackend.ReverseProxy
                       ▼
            [ReverseProxy (proxy.go)]
                       │  Attaches X-Forwarded headers & proxies TCP
                       ▼
        [SimulatedBackend (backend.go:8081)]
                       │  Processes & returns HTTP 200 OK
                       ▼
[CLIENT] ◄──────── 200 OK Response "Hello from backend server running at 8081!"
```

---

## 💥 Live Failure & Recovery Execution

Jab hum backend 1 ko manual failure state me toggle karte hain (`curl http://localhost:8081/toggle`):

```
1. Developer ──► GET /toggle ──► Backend 1 (:8081) healthy state toggled to FALSE
                                      │
2. Health Checker 2s Ticker ──────────┼──► GET http://127.0.0.1:8081/health
                                      │
3. Backend 1 returns HTTP 500 FAIL ───┘
                                      │
4. Health Checker calls ──────────────┼──► b1.SetAlive(false) (Write Lock)
                                      │
5. Backend 1 marked OFFLINE ──────────┘
                                      │
6. Next Client Request arrives ───────┼──► Gateway NextBackend() skips Backend 1
                                      │
7. Client Request routed ─────────────┴──► Backend 2 (:8082) automatically!
```

---

## 🛑 Graceful Shutdown Flow (`server/server.go`)

Jab developer `Ctrl+C` (SIGINT) press karta hai:

```
┌────────────────────────────────────────────────────────────────────────┐
│                   GRACEFUL SHUTDOWN EXECUTION FLOW                     │
│                                                                        │
│  1. SIGINT / SIGTERM Signal Received by OS Signal Channel              │
│                                │                                       │
│                                ▼                                       │
│  2. gs.checker.Stop() called   ──► Closes stopChan & Waits for WG     │
│                                │                                       │
│                                ▼                                       │
│  3. context.WithTimeout(10s)   ──► Bounded context for request drain   │
│                                │                                       │
│                                ▼                                       │
│  4. httpServer.Shutdown(ctx)   ──► Rejects new HTTP connections &      │
│                                    drains active connections           │
│                                │                                       │
│                                ▼                                       │
│  5. Clean Exit                 ──► 0 Leaked Goroutines! Clean Stop.    │
└────────────────────────────────────────────────────────────────────────┘
```

[server/server.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/server/server.go#L56-L79):
1. `signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)` OS interrupt capture karta hai.
2. `gs.checker.Stop()` call hoke background health ticker ko `stopChan` channel dwara clean end karta hai.
3. `gs.httpServer.Shutdown(ctx)` ongoing active requests ko complete hone ka 10-second timeout window deta hai aur new connections accept karna band kar deta hai.

---

## 🎉 Conclusion & Next Steps

Aapne successfully poora **GoGateway** architecture master kar liya hai!
- **Goroutines** se high-concurrency background execution.
- **`sync.RWMutex` & `atomic`** se 100% thread-safe memory protection.
- **`http.Handler` & Closures** se modular, clean proxying code.

Aap [HindDoc/README.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/HindDoc/README.md) par jaakar kisi bhi specific concept ko index se re-visit kar sakte hain!
