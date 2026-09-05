# 02. Goroutines, Channels & Tickers: Codebase Concurrency ⚡

Is guide me hum dekhenge ki **GoGateway** project me Concurrency (Ek saath multiple tasks run karna) kaise kaam kar rahi hai, **Goroutines** kahan-kahan ban rahi hain, aur **Channels & Tickers** ka real-world setup kya hai.

---

## 🗺️ Codebase Concurrency Overview Map

Hamare Gateway me 4 major concurrency layers chal rahi hain:

```
┌─────────────────────────────────────────────────────────────────────────┐
│              1️⃣ MAIN EXECUTION THREAD (main.go)                        │
│              - Load Flags & Initialize Backend Pool                     │
│              - Wait on OS Signals (SIGINT / SIGTERM)                    │
└────────────────────────────────────┬────────────────────────────────────┘
                                     │ Spawns Goroutines
           ┌─────────────────────────┼─────────────────────────┐
           ▼                         ▼                         ▼
┌───────────────────────┐ ┌────────────────────┐ ┌────────────────────────┐
│  2️⃣ SIMULATED BACKENDS │ │  3️⃣ HEALTH CHECKER  │ │  4️⃣ GATEWAY HTTP SERVER│
│   (backend/backend.go)│ │(health/checker.go) │ │   (server/server.go)   │
│                       │ │                    │ │                        │
│ • Goroutine 1 (:8081) │ │ • 2s Ticker Loop   │ │ • Goroutine (:8080)    │
│ • Goroutine 2 (:8082) │ │ • Parallel Checks  │ │ • Per-Request Goroutine│
│ • Goroutine 3 (:8083) │ │   (B1, B2, B3)     │ │   for Client Traffic   │
└───────────────────────┘ └────────────────────┘ └────────────────────────┘
```

---

## 1. Simulated Backends Ko Goroutines Me Spawning

[main.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/main.go#L30-L36) me hum har downstream backend ko alag goroutine me start karte hain:

```go
// main.go (Lines 30-36)
for _, addr := range cfg.BackendAddrs {
	sb := backend.NewSimulatedBackend(addr)
	simulatedServers = append(simulatedServers, sb)

	// Start backend in a background goroutine!
	go func(srv *backend.SimulatedBackend) {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Main] Simulated backend at %s failed: %v", srv.Addr, err)
		}
	}(sb)
}
```

### ❓ Kyu Kiya Ye? (C++ Comparison)
Agar hum `go` keyword nahi lagate, toh pehla `srv.Start()` method execution ko **block** kar deta (kyunki `http.ListenAndServe()` infinite loop me connections accept karta hai), aur baki 2 backends waise hi rukh jaate!

C++ me aapko `std::thread t(&SimulatedBackend::Start, sb); t.detach();` likhna padta. Go me bas **`go`** keyword add kar do!

> [!IMPORTANT]
> **Closure Parameter Passing Rule**: Notice karo humne `go func(srv *backend.SimulatedBackend)` me argument `(sb)` pass kiya hai. Agar hum loop variable `addr` ya `sb` ko bina argument pass kiye andar use karte, toh loop variables ka race condition ho jata! Passing parameter copies the pointer safely.

---

## 2. Active Health Checking Ticker Loop & Channels

[health/checker.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/health/checker.go#L49-L66) me active polling system ek dedicated goroutine me chalta hai:

```go
// health/checker.go (Lines 49-66)
go func() {
	defer hc.wg.Done()
	ticker := time.NewTicker(hc.interval) // e.g., Har 2 Seconds
	defer ticker.Stop()

	hc.checkAll() // Startup par immediate health check

	for {
		select {
		case <-ticker.C:
			hc.checkAll() // Har 2 seconds par saare backends query karo
		case <-hc.stopChan:
			log.Println("[HealthChecker] Stopping active background checks...")
			return // Goroutine exit!
		}
	}
}()
```

### 🔍 Deep Dive Concepts:

1. **`time.NewTicker(interval)`**:
   Ye ek internal timer hai jo har 2 second par channel `ticker.C` me current time send karta hai.
2. **`select` Statement**:
   C++ ke `switch` statement ki tarah, lekin **Channels** ke liye!
   `select` block me jo bhi channel sabse pehle ready hota hai (chaye `ticker.C` ho ya shutdown signal `stopChan`), uski `case` execute hoti hai.
3. **`stopChan chan struct{}`**:
   - `struct{}` Go me **Zero Bytes** memory leta hai!
   - Signal dene ke liye memory space waste karne ki zaroorat nahi hoti. When we call `close(hc.stopChan)`, saare waiting select blocks ko unblock signal mil jata hai!

---

## 3. Parallel Backend Checking with `sync.WaitGroup`

Jab 2 seconds poore hote hain, `hc.checkAll()` call hota hai. Lekin agar hamare paas 100 backends hote, toh kya hum ek-ek karke sequential check karte? NO! Hum unko bhi **concurrent goroutines** me run karte hain!

[health/checker.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/health/checker.go#L85-L98):

```go
func (hc.HealthChecker) checkAll() {
	backends := hc.pool.GetBackends()
	var checkWg sync.WaitGroup // Counter for synchronization

	for _, b := range backends {
		checkWg.Add(1) // Counter + 1
		go func(backend *loadbalancer.Backend) {
			defer checkWg.Done() // Execution khatam hone par Counter - 1
			hc.checkBackend(backend)
		}(b)
	}

	checkWg.Wait() // Jab tak Counter 0 na ho jaye, tab tak ruko!
}
```

```
┌────────────────────────────────────────────────────────────────────────┐
│                      HEALTH CHECKER TICKER LOOP                        │
│                      sync.WaitGroup Counter = 3                        │
└──────────────┬───────────────────┬───────────────────┬─────────────────┘
               │                   │                   │
               ▼                   ▼                   ▼
    ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
    │  Goroutine B1    │  │  Goroutine B2    │  │  Goroutine B3    │
    │ GET /health      │  │ GET /health      │  │ GET /health      │
    └──────────┬───────┘  └──────────┬───────┘  └──────────┬───────┘
               │ 200 OK              │ 200 OK              │ 500 FAIL
               ▼                     ▼                     ▼
        checkWg.Done()        checkWg.Done()        checkWg.Done()
               └─────────────────────┼─────────────────────┘
                                     ▼
                          checkWg.Wait() Unblocks!
```

### C++ Comparison:
C++ me aisi fan-out fan-in concurrency karne ke liye `std::future`, `std::async`, ya thread pools manage karne padte. Go me `sync.WaitGroup` se 5 lines me clean parallel execution ho jata hai!

---

## 4. `net/http` Automatic Per-Connection Goroutines

Go Gateway ki sabse badi power ye hai ki jab multiple clients (e.g., 5,000 HTTP Requests/sec) `:8080` par aate hain, toh **Go ka standard `net/http` package har incoming client TCP connection ke liye NAYI GOROUTINE create kar deta hai!**

[server/server.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/server/server.go#L44-L49):

```go
go func() {
	log.Printf("[GatewayServer] Gateway listening on %s", gs.httpServer.Addr)
	if err := gs.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		serverErrors <- err
	}
}()
```

Go standard library internally karta hai:
```go
// Inside net/http internal code (Conceptual)
for {
    rw, err := l.Accept() // Wait for TCP Client
    c := newConn(rw)
    go c.serve(connCtx)   // Direct new goroutine for EVERY HTTP request!
}
```

Isi wajah se har HTTP Request concurrent thread par chalti hai — aur yahi se paida hota hai **Race Condition** ka khatra, jisko hum Next Chapter me cover karenge!
