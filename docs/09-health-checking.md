# 09. Active and Passive Health Checking

## 1. CONCEPT
A load balancer must ensure traffic is only routed to healthy backend servers. **Health Checking** is the process of monitoring backend servers to detect failures and recoveries, automatically adjusting the routing pool.

---

## 2. WHY IT EXISTS
Downstream servers fail due to software bugs, hardware crashes, network partitions, or resource exhaustion.
* If a load balancer continues routing requests to a dead server, clients will experience connection failures or timeouts.
* The system must detect failures automatically, remove the failed node from the pool, and reintroduce it once it recovers.

---

## 3. HOW IT WORKS
There are two primary paradigms of health checking:
1. **Active Health Checking (Background Polling)**:
   * The load balancer periodically sends dedicated "ping" or HTTP GET requests to a specific endpoint (e.g. `/health`) on each backend.
   * If a backend fails to respond within a timeout or returns a non-200 code, it is marked dead.
2. **Passive Health Checking (Traffic-driven Detection)**:
   * The load balancer monitors real client traffic moving through the proxy.
   * If a forwarded client request fails (e.g. connection refused, read timeout), the load balancer instantly intercepts the failure and marks the backend dead without waiting for the next active health check.

---

## 4. INTERNALS: HEALTH STATE TRANSITIONS
A backend server transitions through a simple state machine:

```
          ┌──────────────────────────────────┐
          │                                  │ (Active check passes)
          ▼                                  │
   ┌──────────────┐   Active/Passive Fail  ┌──────────────┐
   │    ONLINE    │ ─────────────────────► │   OFFLINE    │
   │  (Receiving) │                        │ (Excluded)   │
   └──────────────┘                        └──────────────┘
          ▲                                       │
          └───────────────────────────────────────┘
                     (Active check passes)
```

### In-Flight Requests on Failure
When a backend is marked offline:
* **Existing connections**: Requests already in flight on that backend continue processing. They are not interrupted by the status change.
* **New connections**: All subsequent requests bypass the unhealthy backend immediately.
* **Graceful draining**: In production, the backend is allowed to finish handling existing requests before being completely detached.

---

## 5. PROJECT USAGE
We implement both:
* **Active Health Checking**: Inside `health/checker.go`, running a background goroutine loop checking all backends every $N$ seconds.
* **Passive Health Checking**: Inside the reverse proxy `ErrorHandler` in `proxy/proxy.go`. If request forwarding fails, we run a callback closure marking that backend offline immediately.

---

## 6. CODE WALKTHROUGH
The core polling logic in `health/checker.go`:

```go
package health

import (
	"net/http"
	"time"
)

type Target struct {
	URL   string
	Alive bool
}

func PerformHealthCheck(t *Target, client *http.Client) {
	resp, err := client.Get(t.URL + "/health")
	if err != nil {
		t.Alive = false // Mark offline on network/timeout error
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Alive = true // Mark healthy on HTTP 200
	} else {
		t.Alive = false // Mark unhealthy on any other status code
	}
}
```

---

## 7. RUNTIME FLOW
```
[ HealthChecker Ticker (5s) ]
            │
            ▼
[ checkAll() goroutine ]
            │
            ├─► Spawn checkBackend(Backend A) ──► GET http://:8081/health ──► Status 200 OK ─► SetAlive(true)
            └─► Spawn checkBackend(Backend B) ──► GET http://:8082/health ──► Connection Refused ─► SetAlive(false)
```

---

## 8. FAILURE CASES
* **Flapping Backends**: A server under heavy load might pass one health check, fail the next, and pass again, causing the load balancer to repeatedly add and remove it. This triggers configuration churn.
   * *Mitigation*: Implement a state transition threshold. For example, mark a backend offline only after 3 consecutive failures, and mark it online only after 3 consecutive successful checks.
* **Slow Checks Blocking Schedulers**: If a backend is unresponsive, a health check request can hang, holding a goroutine and connection open.
   * *Code Mitigation*: We construct our `http.Client` with a strict, short timeout (`HealthCheckTimeout = 2s`), ensuring checks terminate quickly.

---

## 9. TRADEOFFS
### Active Health Checking vs. Passive Health Checking
* **Active Polling**:
  * *Pros*: Detects failures before actual client traffic hits the server; can verify specialized metrics (e.g. database connectivity).
  * *Cons*: Adds network overhead (log polling traffic); latency is bounded by the check interval (e.g., if a server dies 1s after a check, it will receive bad traffic for 4s until the next check).
* **Passive Traffic Monitoring**:
  * *Pros*: Instant failure detection under active client load; zero background traffic overhead.
  * *Cons*: Requires actual clients to experience a failure first before a node is marked offline; cannot detect recovery (must rely on active checks to reintroduce the node).

---

## 10. INTERVIEW QUESTIONS
1. **Q**: What is the difference between active and passive health checks?
   * **A**: **Active health checks** periodically send dedicated probes (ping/GET) to downstream backends to verify status. **Passive health checks** monitor real inline client requests and mark a backend dead immediately if it fails to respond to a routed request.
2. **Q**: How do you prevent health checks from overloading backends?
   * **A**: Set a reasonable polling interval (e.g. 5 to 15 seconds) and establish a lightweight `/health` endpoint on the backend that returns a cached status or performs very simple checks, rather than running heavy database queries on every poll.
3. **Q**: What happens to client requests currently in flight when a backend fails?
   * **A**: In-flight requests will either succeed if they complete before the failure, or terminate with an error (e.g., connection reset). Our reverse proxy catches these forwarding failures, redirects the client to a `502 Bad Gateway`, and instantly triggers passive failure detection to remove the backend from the pool.
