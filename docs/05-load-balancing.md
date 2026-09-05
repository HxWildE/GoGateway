# 05. Load Balancing Design Patterns

## 1. CONCEPT
A **Load Balancer** sits between clients and a backend pool. Its primary job is to distribute incoming network traffic across multiple servers to prevent any single server from becoming a bottleneck, ensuring high availability and fault tolerance.

---

## 2. WHY IT EXISTS
In systems engineering, a single machine has physical scaling limits (CPU, RAM, network bandwidth).
* **Scale-Up (Vertical Scaling)**: Adding resources to one machine. This is expensive and creates a single point of failure (SPOF).
* **Scale-Out (Horizontal Scaling)**: Adding more cheap machines. A load balancer is *required* to distribute traffic across these machines transparently to the client.

---

## 3. HOW IT WORKS
A load balancer maintains a list of backend destinations, known as the **Backend Pool**.
1. **Request Interception**: The load balancer accepts an HTTP request.
2. **Algorithm Execution**: It selects an active backend based on routing algorithms (e.g. Round Robin, Least Connections).
3. **Health Validation**: It verifies if the selected backend is alive. If not, it falls back to check another.
4. **Traffic Forwarding**: It delegates execution to the proxy handler pointing to that backend.

---

## 4. INTERNALS
### Backend Pool States
The backend pool must track the status of its members. Each backend in the pool has:
* **Static Config**: Target Hostname/IP, Port, and Weight.
* **Dynamic State**: IsAlive (boolean), Active Connections Count, and Latency stats.

Because multiple client request goroutines read this state concurrently, and background health check goroutines write to it, all read/write accesses to the pool state **must** be synchronized using locking mechanisms (e.g. Mutexes).

---

## 5. PROJECT USAGE
In our system, we define the pool structure in `loadbalancer/roundrobin.go`. We store a slice of `*Backend` pointers inside a `BackendPool` struct. The pool exposes `NextBackend()` which implements the selection logic thread-safely.

---

## 6. CODE WALKTHROUGH
Our backend pool matches this abstraction structure:

```go
package loadbalancer

import (
	"net/url"
	"sync"
)

type Backend struct {
	URL   *url.URL
	Alive bool
	mu    sync.RWMutex // Protects Alive field
}

type BackendPool struct {
	backends []*Backend // Slice of backends representing the pool
	// Selection state variables
}

// AddBackend registers a new server into the pool.
// Note: We don't need locking for appending during initialization (main.go),
// but we use locking on individual backends when updating states dynamically.
func (bp *BackendPool) AddBackend(b *Backend) {
	bp.backends = append(bp.backends, b)
}
```

---

## 7. RUNTIME FLOW
```
Incoming Client Request
         │
         ▼
[ GatewayHandler.ServeHTTP ]
         │
         ▼
  [ Pool.NextBackend() ] ───► Locks Pool Read-Lock
         │                    Iterates candidates
         │                    Checks candidate.IsAlive()
         ▼
[ Selected Backend Pointer ] ◄─ Returns target
         │
         ▼
[ Forward Request via Proxy ]
```

---

## 8. FAILURE CASES
* **All Backends Unhealthy**: If every backend in the pool fails, `NextBackend()` returns `nil`.
  * *Code Mitigation*: In `GatewayHandler.ServeHTTP`, we verify the return value. If `nil`, we write an HTTP `503 Service Unavailable` response back to the client, preventing Nil Pointer panics.
* **Thundering Herd Problem**: If a backend recovers and is suddenly reintroduced, all queued or subsequent requests might flood it, crashing it again.
  * *Mitigation*: In production, load balancers implement "Slow Start" phases, routing only a small fraction of traffic to a newly recovered backend and ramping it up gradually.

---

## 9. TRADEOFFS
### Comparison of Load Balancing Algorithms
1. **Round Robin**:
   * *Pros*: Simple to implement; extremely low CPU overhead; fair distribution if requests have similar processing costs.
   * *Cons*: Assumes all backends have identical hardware capabilities; doesn't account for varying request weights.
2. **Least Connections**:
   * *Pros*: Routes traffic to the least busy server; ideal for requests with highly variable durations (e.g. long database queries).
   * *Cons*: Requires tracking connection states (higher memory/CPU overhead); sensitive to slow backend failures (if a backend fails instantly, it might have 0 connections, causing the load balancer to flood it with requests).
3. **IP Hash**:
   * *Pros*: Ensures a specific client is always routed to the same backend (Session Stickiness).
   * *Cons*: Uneven distribution if many clients sit behind a single NAT proxy.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: What is a Backend Pool in a load balancer?
   * **A**: A Backend Pool is a logical grouping of downstream servers that are configured to receive forwarded traffic from the load balancer. The pool tracks each server's network address, weight, and runtime health status.
2. **Q**: Compare Round Robin and Least Connections. When would you choose one over the other?
   * **A**: **Round Robin** routes traffic sequentially. It is best when downstream backends have equal hardware specs and request durations are short and uniform. **Least Connections** routes traffic to the server with the fewest active TCP connections. It is preferred when requests take highly variable processing times (e.g., reports vs static files) or backends have different hardware capacities.
3. **Q**: What happens to client requests in flight when a backend is marked unhealthy?
   * **A**: Active connections in flight will either succeed (if the backend finishes processing before failing completely) or terminate abruptly with a TCP reset or timeout. Standard load balancers try to drain existing connections gracefully while preventing any *new* incoming requests from being routed to that backend.
