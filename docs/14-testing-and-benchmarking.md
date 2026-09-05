# 14. Testing and Benchmarking Network Services

## 1. CONCEPT
In high-performance networking, software correctness and latency bounds must be verified. Go provides built-in tools for **Unit Testing**, **HTTP Mocking**, and **Micro-benchmarking** directly in its `testing` standard package.

---

## 2. WHY IT EXISTS
* **Testing**: Ensures our round-robin selections, header rewrites, and failovers work correctly and don't break during refactoring.
* **HTTP Mocking (`httptest`)**: Allows us to simulate live web clients and backend servers locally without binding to real external network interfaces, preventing test flakiness and port clashes.
* **Benchmarking**: Measures the throughput (operations per second) and memory allocations of our hot paths (e.g. `NextBackend()`), ensuring we don't introduce performance regressions.

---

## 3. HOW IT WORKS

### Go Testing Basics
Test files must end with `_test.go` and reside in the same package directory as the code being tested (or a separate test package). Test functions must match the signature:
`func TestName(t *testing.T)`

### Mocking HTTP with `httptest`
* **`httptest.NewServer`**: Spins up a local, real HTTP server listening on a random loopback port (e.g. `127.0.0.1:49213`). Excellent for testing proxy forwarding.
* **`httptest.NewRecorder`**: An implementation of `http.ResponseWriter` that records status codes, headers, and body bytes written by a handler. Allows direct invocation of `ServeHTTP` without launching a real network server.

---

## 4. INTERNALS: BENCHMARKING WITH `testing.B`
Benchmark functions match the signature:
`func BenchmarkName(b *testing.B)`

The Go runtime runs the benchmark function repeatedly, adjusting the loop counter `b.N` until it collects a statistically stable measurement.
We run benchmarks using:
`go test -bench=. -benchmem ./...`

### Interpreting Benchmark Outputs
* **`ns/op`**: Nanoseconds per operation (lower is better).
* **`B/op`**: Bytes allocated per operation. High bytes indicate heap allocations causing Garbage Collection (GC) pauses.
* **`allocs/op`**: Number of heap allocations per operation. Our load balancer selection path should aim for `0 allocs/op`.

---

## 5. PROJECT USAGE
We implement three test suites under `tests/`:
1. `roundrobin_test.go`: Tests selection distribution, failure exclusions, and concurrency safety.
2. `proxy_test.go`: Uses `httptest` to test header modifications and failure triggers.
3. `integration_test.go`: Links the components and verifies end-to-end routing.

---

## 6. CODE WALKTHROUGH
An example of micro-benchmarking our Round-Robin selection logic:

```go
package tests

import (
	"net/url"
	"testing"

	"letsgolang/loadbalancer"
)

func BenchmarkNextBackend(b *testing.B) {
	pool := loadbalancer.NewBackendPool()
	urlA, _ := url.Parse("http://127.0.0.1:8081")
	urlB, _ := url.Parse("http://127.0.0.1:8082")

	pool.AddBackend(&loadbalancer.Backend{URL: urlA, Alive: true})
	pool.AddBackend(&loadbalancer.Backend{URL: urlB, Alive: true})

	b.ResetTimer() // Reset timer to ignore setup time

	for i := 0; i < b.N; i++ {
		_ = pool.NextBackend() // Execute path under test
	}
}
```

---

## 7. RUNTIME FLOW
```
[ go test -bench ]
        │
        ├─── Run BenchmarkNextBackend with b.N = 100 ──► takes 200ns (too fast)
        ├─── Run BenchmarkNextBackend with b.N = 10000 ──► takes 15µs
        ├─── Run BenchmarkNextBackend with b.N = 5000000 ──► stabilized
        ▼
[ Output stats: 2.1 ns/op, 0 B/op, 0 allocs/op ]
```

---

## 8. FAILURE CASES
* **Flaky Ports in Tests**: Hardcoding ports (like `8080`) in tests causes them to fail if the port is already bound on the testing machine.
  * *Code Mitigation*: Always use `httptest.NewServer()`, which binds dynamically to port `:0`, meaning the OS automatically assigns a free loopback port.
* **Concurreny Data Races in Tests**: Tests that verify thread-safety might pass but hide race conditions.
  * *Mitigation*: Always run tests with `go test -race ./...` in environments supporting CGO.

---

## 9. TRADEOFFS
### Local Benchmarking vs. Production Load Testing
* **Micro-benchmarking (`testing.B`)**:
  * *Pros*: Extremely fast; measures raw execution code logic; checks heap allocations directly.
  * *Cons*: Excludes operating system network stack bottlenecks, TCP handshakes, CPU scheduling latency, and physical network card throughput.
* **Production Load Testing (e.g. using `wrk` or `hey`)**:
  * *Pros*: Tests the entire stack (TCP kernel sockets, proxy parsing, network interfaces, downstream latencies) under realistic load.
  * *Cons*: Requires complex setup; results are affected by system background processes.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: How do you mock HTTP requests and responses in Go unit tests?
   * **A**: I use the standard library `net/http/httptest` package. For testing handlers directly, I use `httptest.NewRequest` to generate mock HTTP requests and `httptest.NewRecorder` to capture output bytes and statuses. For full integration tests, I spin up lightweight test servers using `httptest.NewServer`.
2. **Q**: What are heap allocations in benchmarks, and why do they matter for network servers?
   * **A**: Heap allocations (measured in `B/op` and `allocs/op` during benchmarks) occur when variables escape the stack to the heap, requiring garbage collection. High allocations trigger GC pauses, which temporarily freeze thread execution, causing latency spikes (p99 latency degradation) in high-throughput network servers.
3. **Q**: How does `testing.B` determine the value of `b.N`?
   * **A**: The benchmark runner starts with `b.N = 1`. It runs the loop and measures execution time. If it finishes too quickly, it increases `b.N` (typically scaling 1, 2, 5, 10, 20, 50...) and runs again, repeating the process until the benchmark execution takes at least 1 second to gather stable average timings.
