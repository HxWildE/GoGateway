# Upgrade 2: Observability (Prometheus Metrics & pprof)

## Kyun karna hai? (Why?)
"Blind" proxy production me maut ke barabar hai. Agar proxy me memory leak ho raha hai, ya achanak RPS (Requests per second) badh gaya, toh tumhe kaise pata chalega? 
* **pprof**: Go ka inbuilt profiler hai jo batata hai ki CPU aur Memory kahan waste ho rahi hai.
* **Prometheus Metrics**: Batata hai kitni requests aayi, kitni fail hui. 
Interviewer ko yeh bataoge toh seedha Senior ya Strong Mid-level SDE ki category me jaoge.

## Implementation Plan (How to do it)

### Part A: Adding `pprof` (Extremely Easy)
Go me pprof add karna literally 1 line ka kaam hai because it's built into the standard library.

1. **Import pprof in `server.go` or `main.go`**:
   ```go
   import _ "net/http/pprof"
   ```
   *Underscore `_` ka matlab hai hum pprof ka init function run karwana chahte hain jo default HTTP mux me apne routes attach kar deta hai.*

2. **Expose pprof on a separate port**:
   Gateway ka port `:8080` public client traffic ke liye hai. Pprof ko hamesha private/internal port (e.g., `:6060`) par chalana chahiye so hackers CPU data na dekh sakein.
   
   Add this in `main.go` inside a goroutine:
   ```go
   go func() {
       log.Println("Starting pprof on :6060")
       log.Println(http.ListenAndServe("localhost:6060", nil)) // DefaultServeMux used by pprof
   }()
   ```

3. **How to test it during interview**:
   Open browser at `http://localhost:6060/debug/pprof/`. Tumhe wahan running goroutines, heap (memory), aur CPU usage dikh jayega. "Sir, I exposed a private debug port for live heap analysis to catch memory leaks." -> Instant impress!

---

### Part B: Basic Custom Metrics (Without Prometheus Dependency)
Agar external dependencies (like prometheus client library) strictly avoid karni hai, toh hum ek custom `/metrics` endpoint bana sakte hain.

1. **Add Counters in GatewayHandler**:
   `proxy/proxy.go` me:
   ```go
   import "sync/atomic"

   type GatewayMetrics struct {
       TotalRequests uint64
       FailedRequests uint64
   }
   var Metrics GatewayMetrics
   ```

2. **Increment Counters on Request**:
   Inside `ServeHTTP` function:
   ```go
   atomic.AddUint64(&Metrics.TotalRequests, 1)
   
   // Agar TargetBackend nil hai ya fail ho jaye:
   // atomic.AddUint64(&Metrics.FailedRequests, 1)
   ```

3. **Expose Metrics Endpoint**:
   Modify `server.go` to handle a special route, OR spin up another admin server:
   ```go
   go func() {
       adminMux := http.NewServeMux()
       adminMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
           reqs := atomic.LoadUint64(&proxy.Metrics.TotalRequests)
           fails := atomic.LoadUint64(&proxy.Metrics.FailedRequests)
           w.Write([]byte(fmt.Sprintf("Total Requests: %d\nFailed Requests: %d\n", reqs, fails)))
       })
       http.ListenAndServe("localhost:9090", adminMux)
   }()
   ```

### Final Interview Flex
"Instead of using heavy 3rd party libraries, I exposed my own atomic counters on an isolated internal Admin port (`:9090`) to track throughput and error rates, keeping the binary zero-dependency and lightweight."
