# 02. Concurrency, Goroutines & Runtime Execution Flow

```
========================================================================================
                          GOROUTINES & CONCURRENCY MAP
========================================================================================

                 [ OS Process: GoGateway (PID 12450) ]
                                   │
               ┌───────────────────┴───────────────────┐
               ▼                                       ▼
    ┌────────────────────────┐              ┌────────────────────────┐
    │  GOROUTINE 1: main()   │              │  Go Runtime Scheduler  │
    │  (Coordinator Thread)  │              │  (GMP Model: 8 Cores)  │
    └──────────┬─────────────┘              └────────────────────────┘
               │
               │  go sb.Start() (Spawns N backends)
               ├──────────────────────────────────────────────┐
               │                                              │
               ▼                                              ▼
    ┌────────────────────────┐                     ┌────────────────────────┐
    │ GOROUTINE 2: :8081     │                     │ GOROUTINE 3: :8082     │
    │ SimulatedBackend 1     │                     │ SimulatedBackend 2     │
    └────────────────────────┘                     └────────────────────────┘
               │
               │  go checker.Start()
               ├──────────────────────────────────────────────┐
               │                                              │
               ▼                                              ▼
    ┌────────────────────────┐                     ┌────────────────────────┐
    │ GOROUTINE 4: Health    │                     │ GOROUTINE 5: Server    │
    │ time.Ticker Loop (5s)  │                     │ http.ListenAndServe()  │
    └──────────┬─────────────┘                     └──────────┬─────────────┘
               │                                              │
               │  Spawns per probe                            │  Spawns per incoming HTTP req
               ▼                                              ▼
    ┌────────────────────────┐                     ┌────────────────────────┐
    │ GOROUTINE 6..N: Probe  │                     │ GOROUTINE N+1: Worker  │
    │ Worker (GET /healthz)  │                     │ ServeHTTP(w, r)        │
    └────────────────────────┘                     └────────────────────────┘
```

---

## 🚦 Goroutine Lifecycle & Graceful Shutdown Diagram

```mermaid
stateDiagram-v2
    [*] --> Initializing : main() starts

    state "Simulated Backends" as SB {
        [*] --> BackendRunning : go sb.Start()
        BackendRunning --> BackendClosed : srv.Close() on defer
    }

    state "Health Checker" as HC {
        [*] --> TickerLoop : go checker.Start()
        TickerLoop --> ProbeBackends : <-ticker.C (Every 5s)
        ProbeBackends --> TickerLoop : Update Backend.SetAlive()
        TickerLoop --> CheckerStopped : <-stopChan
    }

    state "Gateway Server" as GS {
        [*] --> Listening : srv.Start()
        Listening --> TrapSignal : os.Interrupt (SIGINT / Ctrl+C)
        TrapSignal --> Draining : server.Shutdown(ctx) [10s Timeout]
        Draining --> ShutdownComplete
    }

    ShutdownComplete --> [*] : Exit 0 (Clean Memory)
```

---

## ⚡ The Go GMP Runtime Model (Why It Beats C++ Threads)

```
+-----------------------------------------------------------------------------+
| G (Goroutines)  : 100,000+ lightweight user-space threads (~2KB stack)      |
| P (Processors)  : GOMAXPROCS logical execution contexts (matches CPU cores) |
| M (Machines)    : Actual OS kernel threads managed by Linux / Windows kernel|
+-----------------------------------------------------------------------------+

   [ G1 ] [ G2 ] [ G3 ] [ G4 ] [ G5 ]  <--- Local Run Queue
            │
            ▼
        ┌───────┐
        │   P   │  (Logical Processor)
        └───┬───┘
            ▼
        ┌───────┐
        │   M   │  (OS Kernel Thread) ──► [ Hardware CPU Core 0 ]
        └───────┘

💡 Work Stealing: When P1 has no Goroutines left, it steals 50% of Gs from P2!
```
