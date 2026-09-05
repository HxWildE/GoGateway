# TI Interview Master Guide: 100 CN, OS, Load Balancer & Rate Limiter Questions

> **Context for TI (Texas Instruments) & Core Systems Interviews**:
> TI and hardware/systems companies evaluate candidates deeply on **Computer Networks (CN)**, **OS Internals (Sockets, Epoll, Mutexes, Context Switching)**, and **Low-Level Software Architecture**. They want to know *how things work down to the byte and kernel level*.

---

## 🎯 Top 15 Things You MUST Know to Prove You Are a Top-Tier Candidate

Agar tumne yeh project resume me daala hai, toh interviewer ke pehle 10 minute me yeh 15 points tumhari command prove karenge:

1. **L4 vs L7 Difference at Socket Layer**: L4 forwards raw TCP packets via NAT/DSR without reading payload (1 TCP session or packet rewriting). L7 terminates the incoming TCP connection, parses HTTP/TLS buffers, and opens a 2nd TCP connection to the backend.
2. **Reverse Proxy vs Forward Proxy**: Forward proxy masks client identity from origin servers (outbound); Reverse proxy masks backend infrastructure and coordinates load balancing/TLS (inbound).
3. **Atomic Operations vs Mutex in Load Balancing**: How `sync/atomic.AddUint64` provides lock-free cursor increments for Round Robin vs Mutex locking overhead under heavy CPU core contention.
4. **Active vs Passive Health Checking**: Active = periodic background probe (`/healthz` timer); Passive = inline reactive failure detection via reverse proxy error callbacks/closures (`502/Connection Refused`).
5. **Token Bucket Lazy Refill Formula**: `tokens = min(capacity, current + elapsed * refill_rate)`. Why background worker tickers are an anti-pattern for millions of users ($O(1)$ memory vs $O(N)$ active timers).
6. **Fixed Window Boundary Bug (2x Spike)**: Why bursts across window boundaries (e.g. 1:59 and 2:01) can overwhelm downstream backends, and why Sliding Window Counter solves it with 2 simple integer counters.
7. **Distributed Rate Limiting Race Conditions**: Why Redis `GET` + `INCR` has a Check-Then-Set race condition and how Redis **Lua scripts** guarantee atomic execution.
8. **Consistent Hashing with Virtual Nodes**: Why modulo hashing ($K \pmod N$) causes massive cache invalidation on server addition/removal, and how the $0 \dots 2^{32}-1$ hash ring with 150 virtual nodes balances shards with only $1/N$ key migration.
9. **TCP 3-Way Handshake & Connection Pooling**: How `Keep-Alive` and connection reuse avoid paying the 3-way handshake (SYN, SYN-ACK, ACK) and TLS 1.3 handshake latency on every single HTTP request.
10. **File Descriptors & `epoll` / Go `netpoller`**: How a single OS process can handle 100,000+ concurrent connections without creating 100,000 OS threads (Non-blocking I/O + Edge/Level-triggered event loops).
11. **HTTP Status Codes & Rate Limit Headers**: `429 Too Many Requests`, `502 Bad Gateway`, `503 Service Unavailable`, `504 Gateway Timeout`, along with `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, and `Retry-After`.
12. **TCP `TIME_WAIT` & Ephemeral Port Exhaustion**: What happens when a reverse proxy opens/closes short-lived connections rapidly (65,535 source ports exhausted due to 2*MSL 60s timeout).
13. **Circuit Breaker Pattern**: State machine (`Closed` $\rightarrow$ `Open` $\rightarrow$ `Half-Open`) to prevent cascading failures when a backend dependency degrades.
14. **Direct Server Return (DSR)**: Layer 4 optimization where incoming packets go through LB, but egress high-bandwidth traffic bypasses the LB directly to the client via MAC layer address translation.
15. **Clock Drift in Distributed Systems**: How NTP clock discrepancies between servers can cause distributed rate limiters or token expirations to behave inconsistently.

---

## 📚 100 Interview Questions & Conceptual Answers

### Part 1: Computer Networks & Socket Layer (Q1 - Q25)

**Q1: What happens at the networking layer when a client types a URL in the browser?**
- DNS lookup (Browser cache $\rightarrow$ OS cache $\rightarrow$ Resolving Name Server $\rightarrow$ Root/TLD $\rightarrow$ Authoritative).
- TCP 3-Way Handshake (SYN, SYN-ACK, ACK) on IP:Port.
- TLS Handshake (ClientHello, ServerHello, Certificate, Key Exchange, Finished).
- HTTP Request sent $\rightarrow$ Reverse Proxy / LB parses $\rightarrow$ Routes to Backend $\rightarrow$ HTTP Response returned $\rightarrow$ TCP Teardown (FIN/ACK).

**Q2: What is the TCP 3-Way Handshake and why can't it be a 2-Way Handshake?**
- Client $\rightarrow$ Server: `SYN` (seq=x). Server $\rightarrow$ Client: `SYN-ACK` (seq=y, ack=x+1). Client $\rightarrow$ Server: `ACK` (ack=y+1).
- 2-way cannot prevent duplicate old connection requests from establishing false sessions if packets are delayed in the network.

**Q3: What is the difference between TCP and UDP?**
- TCP: Connection-oriented, reliable, in-order delivery, flow control (sliding window), congestion control (AIMD/Cubic).
- UDP: Connectionless, lightweight, no retransmissions, no ordering, zero handshake latency (DNS, VoIP, Gaming, QUIC/HTTP3).

**Q4: What is OSI Layer 4 vs Layer 7?**
- Layer 4 (Transport): Knows IP and Port (TCP/UDP). Cannot see URLs, JSON, Cookies.
- Layer 7 (Application): Decodes protocol stream (HTTP, WebSocket, gRPC). Understands headers, paths, payload.

**Q5: What is TLS Termination in a Load Balancer?**
- Load Balancer decrypts HTTPS traffic from the client using SSL certificates. Traffic from LB to internal backend servers travels over fast, unencrypted HTTP (or internal mTLS) inside a private VPC.

**Q6: What is HTTP Keep-Alive and why is it crucial for Load Balancers?**
- Persistent TCP connections that allow multiple HTTP requests/responses over the same TCP socket, eliminating recurring handshake latency.

**Q7: What is the TCP `TIME_WAIT` state and how does it affect a reverse proxy?**
- After closing a TCP connection, the initiator stays in `TIME_WAIT` for $2 \times \text{MSL}$ (Maximum Segment Lifetime, ~60s) to ensure delayed packets don't corrupt a new connection.
- High-throughput proxies without connection pooling exhaust ephemeral ports (65,535 limit) and fail with `EADDRNOTAVAIL`.

**Q8: What is HTTP/1.1 vs HTTP/2 vs HTTP/3?**
- HTTP/1.1: Text-based, Head-of-Line (HoL) blocking at application layer.
- HTTP/2: Binary framing, multiplexing multiple streams over 1 TCP connection, header compression (HPACK). Still suffers from TCP HoL blocking on packet loss.
- HTTP/3: Runs over QUIC (UDP), solving TCP-level HoL blocking and enabling 0-RTT handshakes.

**Q9: What is `X-Forwarded-For` and `X-Real-IP`?**
- When a reverse proxy forwards a request, the backend sees the proxy's IP as `RemoteAddr`. The proxy adds `X-Forwarded-For: <client_ip>, <proxy1_ip>` so backends know the real client IP.

**Q10: What is MTU and MSS?**
- MTU (Maximum Transmission Unit): Max packet size (usually 1500 bytes on Ethernet).
- MSS (Maximum Segment Size): MTU minus IP header (20B) and TCP header (20B) = 1460 bytes payload.

**Q11: What is a Socket, File Descriptor, and Port?**
- Port: 16-bit number identifying a network process.
- File Descriptor (FD): OS integer index referencing an open I/O stream (file, socket, pipe).
- Socket: Kernel abstraction representing an endpoint of network communication `(Source IP, Source Port, Dest IP, Dest Port, Protocol)`.

**Q12: How does `epoll` / `kqueue` differ from `select` / `poll`?**
- `select`/`poll`: $O(N)$ scanning across all watched FDs on every event loop iteration.
- `epoll` (Linux): $O(1)$ event notification using kernel red-black tree and ready-list. Scales to millions of connections.

**Q13: How does Go handle network I/O with Goroutines?**
- Go uses `netpoller` which integrates with `epoll`/`kqueue`. When a goroutine blocks on a socket read/write, Go parks the goroutine and puts the FD in `netpoller`, freeing the OS thread ($M$) to run other goroutines ($G$).

**Q14: What is DNS Round Robin and why is it not enough for load balancing?**
- DNS returns multiple IPs in rotation.
- Flaws: DNS caching in ISPs/browsers ignores updates; no real-time health checking; can route users to dead servers.

**Q15: What is Anycast Routing?**
- Same IP address advertised by multiple servers in different global locations via BGP (Border Gateway Protocol). Routers send traffic to the topologically closest node (used by Cloudflare, 8.8.8.8).

**Q16: What is TCP Congestion Control (Slow Start, Congestion Avoidance)?**
- Starts with small Congestion Window (`cwnd`), doubles it exponentially per RTT until packet loss/threshold, then grows linearly (AIMD).

**Q17: What is SYN Flood attack and how do SYN Cookies mitigate it?**
- Attacker sends millions of `SYN` packets without sending final `ACK`, exhausting server connection backlog memory (`half-open connections`).
- SYN Cookies: Server encodes connection state inside the initial `SYN-ACK` sequence number without allocating memory until the valid `ACK` arrives.

**Q18: What is Direct Server Return (DSR)?**
- Layer 4 technique where LB rewrites MAC address to backend, but backend replies directly to client IP, preventing LB from becoming an egress bandwidth bottleneck.

**Q19: What is NAT (Network Address Translation) in Load Balancers?**
- LB modifies the Destination IP of incoming packets to the private IP of the chosen backend, and modifies Source IP on the way back (Full NAT / SNAT).

**Q20: What is Head-of-Line (HoL) Blocking?**
- In HTTP/1.1: Request 2 must wait for Request 1 to finish on the same TCP connection.
- In TCP: If packet 1 is lost, packets 2 and 3 wait in the OS buffer even if they arrived safely.

**Q21: What is Nagle's Algorithm and `TCP_NODELAY`?**
- Nagle buffers small packets to send fewer larger segments. For latency-sensitive reverse proxies, disabling Nagle via `TCP_NODELAY` sends packets immediately.

**Q22: What is the difference between Half-Close and Full-Close in TCP?**
- Half-Close: One party sends `FIN` (done writing) but can still read incoming data (`ACK`). Full-Close: Both sides send `FIN`/`ACK`.

**Q23: What is Cross-Origin Resource Sharing (CORS)?**
- Browser security mechanism enforcing that scripts on domain A cannot access resources on domain B without explicit `Access-Control-Allow-Origin` headers.

**Q24: What is Server-Sent Events (SSE) vs WebSockets?**
- SSE: Unidirectional (Server $\rightarrow$ Client) over standard HTTP/1.1 or HTTP/2.
- WebSocket: Full-duplex bidirectional communication protocol initiated via HTTP Upgrade handshake.

**Q25: What is gRPC and how does it route through load balancers?**
- gRPC uses HTTP/2 multiplexing with Protobuf payloads. L4 LBs cannot load balance individual gRPC RPC calls because all calls share one persistent TCP connection; an L7 LB is required.

---

### Part 2: Load Balancer Design & Algorithms (Q26 - Q50)

**Q26: What are the main load balancing algorithms?**
- Round Robin, Weighted Round Robin, Least Connections, Weighted Least Connections, IP Hash, Consistent Hashing, Random, Least Response Time.

**Q27: How does Consistent Hashing work?**
- Maps both servers and request keys onto a $0 \dots 2^{32}-1$ hash ring. Request routes clockwise to the first server hash. When a node leaves or joins, only $K/N$ keys are remapped.

**Q28: Why do we need Virtual Nodes in Consistent Hashing?**
- Without virtual nodes, uneven hash distribution causes "hotspots" where one server handles 80% of the ring. Virtual nodes (e.g. 150 per server) ensure uniform distribution.

**Q29: How do you implement thread-safe Round Robin in Go?**
```go
type RoundRobinPool struct {
    backends []*Backend
    cursor   uint64
}
func (p *RoundRobinPool) Next() *Backend {
    next := atomic.AddUint64(&p.cursor, 1)
    return p.backends[(next-1)%uint64(len(p.backends))]
}
```

**Q30: What is the Thundering Herd Problem in Load Balancing?**
- When a server recovers or cache expires, thousands of queued requests hit that single server simultaneously, crashing it again. Mitigated by jitter, gradual traffic ramp-up (warm-up phase), and circuit breakers.

**Q31: What is Sticky Sessions (Session Affinity) and what are its drawbacks?**
- Routing requests from the same user to the same backend using a cookie or IP hash.
- Drawback: Impairs uniform load distribution; if the backend crashes, user sessions are lost unless stored in a shared session store (Redis).

**Q32: Active vs Passive Health Check trade-offs?**
- Active: Detects dead nodes before user traffic hits them, but incurs continuous probe traffic overhead.
- Passive: Zero overhead on healthy systems, but 1 or more real user requests must fail before a dead backend is discovered. Combining both is best.

**Q33: What is Circuit Breaking in Load Balancers?**
- If failure rate on a backend exceeds a threshold (e.g., 50% errors over 10s), the breaker trips to `OPEN`, immediately failing fast without sending traffic. After a cooldown, it enters `HALF-OPEN` to test a few canary requests.

**Q34: How does a Reverse Proxy handle backpressure?**
- If the client is slow (slow 3G network) and backend is fast, the proxy buffers data up to a limit and then stops reading from the backend socket (`TCP window zero`), preventing memory leaks.

**Q35: What is Connection Pooling in reverse proxies?**
- Maintaining a pool of open, idle TCP connections to backend servers (`MaxIdleConns`, `IdleConnTimeout`) to avoid TCP/TLS handshake latency on every incoming request.

**Q36: What is Canary Deployment vs Blue-Green Deployment?**
- Blue-Green: Two identical environments; LB switches 100% traffic from Blue to Green at once.
- Canary: LB routes 5% traffic to the new version (Canary) and 95% to stable, gradually increasing if error rates remain low.

**Q37: What is Path-Based Routing vs Host-Based Routing?**
- Path-based: `/api/orders` $\rightarrow$ Order Service, `/api/users` $\rightarrow$ User Service.
- Host-based: `orders.app.com` $\rightarrow$ Order Service, `users.app.com` $\rightarrow$ User Service.

**Q38: How do you prevent a Load Balancer from becoming a Single Point of Failure (SPOF)?**
- Active-Passive or Active-Active LB pairs using **VRRP** (Virtual Router Redundancy Protocol) or **Keepalived** with a Floating Virtual IP (VIP), combined with DNS Anycast.

**Q39: What is Global Server Load Balancing (GSLB)?**
- Distributes traffic across multiple geographical data centers using Anycast routing or Geo-DNS based on user proximity and datacenter health.

**Q40: How does NGINX achieve high performance?**
- Master-Worker process architecture: 1 master process, 1 worker per CPU core running an asynchronous non-blocking event loop (`epoll`).

**Q41: What is Envoy Proxy and why is it popular in microservices?**
- High-performance C++ L4/L7 proxy designed for service meshes with dynamic configuration APIs (xDS), built-in observability, and native gRPC/HTTP2 support.

**Q42: What is the difference between Load Balancer and API Gateway?**
- Load Balancer: Pure traffic distribution, health checking, TLS termination.
- API Gateway: Adds business-level capabilities like Authentication (JWT), Rate Limiting, API versioning, request/response transformation, and billing telemetry.

**Q43: What HTTP status code should an LB return if all backends are dead?**
- `503 Service Unavailable` or `502 Bad Gateway`.

**Q44: How does Least Response Time algorithm work?**
- Routes requests to the server with the lowest combination of active connections and average response latency (TTFB).

**Q45: What is Weighted Least Connections?**
- Computes `active_connections / weight` for each server and routes to the server with the lowest ratio.

**Q46: How do you handle graceful shutdown in a Load Balancer?**
1. Stop accepting new connections.
2. Allow active inflight requests to complete within a timeout (e.g. 10s).
3. Close idle backend connection pools.
4. Exit the process.

**Q47: What is IP Hashing and what is its major limitation?**
- `hash(client_ip) % N`.
- Limitation: Thousands of corporate users behind a single NAT gateway share one public IP, sending all their traffic to a single backend node.

**Q48: How does TLS Session Resumption work?**
- Uses Session IDs or Session Tickets (encrypted by server key) so returning clients can resume TLS sessions in 1 RTT instead of 2 RTTs.

**Q49: What is TCP Buffer Tuning in high-throughput proxies?**
- Configuring `SO_RCVBUF` and `SO_SNDBUF` sizes in the OS to match the Bandwidth-Delay Product (BDP = Bandwidth $\times$ RTT).

**Q50: What is Rate Limiting at the Load Balancer level vs Application level?**
- LB level: Rejects malicious traffic at the edge before it consumes backend CPU, threads, or database connections.
- App level: Can enforce complex domain rules (e.g. tier-based user quotas based on DB queries).

---

### Part 3: Rate Limiter Design & Concurrency (Q51 - Q75)

> ⚠️ **NOTE**: Rate Limiting is **NOT** implemented in our GoGateway system. These questions are for conceptual and interview preparation only.

**Q51: What are the 5 classic Rate Limiting algorithms?**
- Token Bucket, Leaky Bucket, Fixed Window Counter, Sliding Window Log, Sliding Window Counter.

**Q52: How does the Token Bucket algorithm handle bursts?**
- If the bucket capacity is $C$ and currently full, $C$ requests can pass immediately with zero delay, after which requests are limited to the refill rate $R$.

**Q53: Why is Lazy Refill preferred over a background Ticker in Token Bucket?**
- Background tickers for 10 million active users would require 10 million active timers or constant loop sweeps, destroying CPU cache. Lazy refill calculates tokens mathematically on arrival: $O(1)$ time and $O(1)$ space.

**Q54: What is the math formula for Token Bucket lazy refill?**
- $\text{tokens} = \min(\text{capacity}, \text{tokens} + (\text{now} - \text{last\_time}) \times \text{refill\_rate})$.

**Q55: What is Leaky Bucket and how does it differ from Token Bucket?**
- Leaky bucket enforces a strictly constant output rate using a queue. Token bucket allows traffic bursts as long as tokens are available.

**Q56: What is the Boundary Problem in Fixed Window Counter?**
- An attacker sends full capacity at $t=59\text{s}$ and again at $t=61\text{s}$, allowing $2\times$ the allowed limit in a 2-second sliding window.

**Q57: How does Sliding Window Log prevent the boundary problem?**
- Stores exact timestamps in a sorted set and removes entries older than $t - \text{window}$. Boundary spikes are impossible because the window slides continuously.

**Q58: Why is Sliding Window Log rarely used for high-throughput public APIs?**
- High memory usage: storing 10,000 timestamps per user for 1 million active users requires gigabytes of RAM.

**Q59: How does Sliding Window Counter approximate the sliding window?**
- $\text{Estimated} = \text{Current Count} + \text{Previous Count} \times \left(\frac{\text{WindowSize} - \text{Elapsed}}{\text{WindowSize}}\right)$.
- Uses only 2 integers per user with $<0.05\%$ error.

**Q60: How do you implement a distributed Rate Limiter with Redis?**
- Store keys with TTL in Redis and execute increment/validation logic atomically using Redis **Lua scripts**.

**Q61: Why is Redis `GET` followed by `INCR` dangerous for rate limiters?**
- Race condition: Two concurrent gateway nodes can both `GET` value 99, see it is below 100, and both execute `INCR`, allowing 101 requests through.

**Q62: What is the advantage of a Redis Lua script?**
- Redis runs Lua scripts as a single atomic unit. No other Redis command can execute in parallel, eliminating check-then-set race conditions without distributed locks.

**Q63: What HTTP status code and headers are returned when rate limited?**
- `HTTP 429 Too Many Requests`.
- `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`.

**Q64: How do you choose the Rate Limit Identifier?**
- Public APIs: IP Address.
- Authenticated APIs: User ID or API Key.
- Sensitive Endpoints: Hybrid (e.g. User ID + Action `login_attempts:user123`).

**Q65: What is the Noisy Neighbor problem and how does rate limiting fix it?**
- One tenant consumes disproportionate system resources, degrading performance for all other tenants. Per-tenant rate limiting isolates and bounds resource usage.

**Q66: How do you handle Rate Limiting across multiple geographically distributed Redis clusters?**
- Centralized sync creates high inter-region latency. Solution: Local in-memory token buckets with periodic asynchronous batch synchronization, or Local Rate Limiting per Region (e.g. 50 req/sec in US-East, 50 req/sec in EU-West).

**Q67: What is Client-Side Rate Limiting?**
- Implementing Exponential Backoff with **Jitter** in client SDKs when receiving `429` or `503` responses to prevent retries from hammering a recovering server.

**Q68: What is Jitter in exponential backoff?**
- Adding randomness to sleep intervals: $t = \text{base} \times 2^{\text{attempt}} + \text{rand}(0, \text{jitter})$. Prevents thousands of clients from retrying simultaneously at the exact same millisecond.

**Q69: What happens if the Redis Rate Limiter cache fails? (Fail-Open vs Fail-Closed)**
- Fail-Open: Allow requests through so business is not interrupted, but log alerts. (Preferred for most e-commerce/SaaS).
- Fail-Closed: Block requests to protect critical internal infrastructure. (Used in high-security banking/financial operations).

**Q70: How does NGINX implement rate limiting?**
- Uses Leaky Bucket (`limit_req_zone`) with a `burst` parameter and optional `nodelay` flag.

**Q71: What is Tiered Rate Limiting?**
- Differentiating limits by user tier (e.g. Free Tier: 60 req/min; Pro Tier: 1,000 req/min; Enterprise: 10,000 req/min).

**Q72: What is Clock Drift and how does it affect distributed rate limiters?**
- Server hardware clocks desynchronize over time. If Server A's clock is 500ms ahead of Server B, timestamp-based sliding windows can miscalculate request counts. Mitigated by PTP/NTP and relying on Redis server time (`TIME` command).

**Q73: How can Rate Limiting protect against Credential Stuffing attacks?**
- Apply strict rate limiting on `/login` and `/reset-password` endpoints (e.g. max 5 attempts per IP per 15 minutes, with CAPTCHA step-up).

**Q74: What is Cost-Based (Weighted) Rate Limiting?**
- Instead of 1 request = 1 token, heavy endpoints cost more tokens (e.g. `GET /user` costs 1 token, `POST /export-large-pdf` costs 20 tokens).

**Q75: What is Concurrency Limiting vs Rate Limiting?**
- Rate Limiting: Bounds requests per unit of time (e.g. 100 req/min).
- Concurrency Limiting: Bounds active simultaneous requests in-flight (e.g. max 10 concurrent database queries per user).

---

### Part 4: OS, Concurrency, Memory & Real-World Failures (Q76 - Q100)

**Q76: What is a Race Condition and how do you detect it in Go?**
- Occurs when two or more goroutines read/write shared memory concurrently without synchronization. Detected using the Go race detector: `go test -race` or `go run -race main.go`.

**Q77: What is the difference between Mutex and RWMutex?**
- `sync.Mutex`: Exclusive lock (only 1 reader OR writer allowed).
- `sync.RWMutex`: Multiple readers can hold `RLock()` simultaneously; `Lock()` is exclusive to a single writer. Ideal for read-heavy workloads (e.g. reading healthy backend pool list).

**Q78: What is Cache Contention / False Sharing in multi-threaded systems?**
- When two CPU cores modify independent variables that happen to share the same 64-byte L1/L2 cache line, forcing cache invalidation cycles across cores. Fixed via struct padding.

**Q79: What is Memory Alignment and Struct Padding in Go / C++?**
- CPUs read memory in 4-byte or 8-byte word boundaries. Arranging struct fields from largest to smallest minimizes padding bytes and saves memory.

**Q80: What is Context Switching and why is it expensive in OS threads?**
- OS saves CPU registers, program counter, and stack pointer, and flushes CPU TLB (Translation Lookaside Buffer). Costs $\approx 1-2\,\mu\text{s}$. Go goroutine switch costs $\approx 10-100\,\text{ns}$ in user space without kernel traps.

**Q81: How does the Go Runtime Scheduler (GMP model) work?**
- **G** (Goroutine): Lightweight green thread (2KB stack).
- **M** (OS Machine Thread): Actual OS thread managed by the kernel.
- **P** (Processor context): Logical resource representing a CPU core (`GOMAXPROCS`) with a local run queue and work-stealing algorithm.

**Q82: What is Work Stealing in the Go Scheduler?**
- When a Processor $P$ runs out of goroutines in its local queue, it steals half the goroutines from another processor's queue to keep all CPU cores busy.

**Q83: What is a Goroutine Leak and how do you prevent it?**
- A goroutine blocked indefinitely on a channel read/write or waiting for a network socket without a timeout, never getting garbage collected. Prevent using `context.WithTimeout` and buffered channels.

**Q84: What is the purpose of `context.Context` in Go network servers?**
- Propagates cancellation signals, deadlines, and request-scoped metadata across API boundaries and child goroutines. When a client disconnects, child database and proxy queries are cancelled immediately.

**Q85: What is Garbage Collection (GC) Stop-The-World (STW) in Go?**
- Go uses a Concurrent Tri-color Mark-and-Sweep collector with sub-millisecond STW phases during mark start and mark termination to ensure low tail latency.

**Q86: How do you achieve Zero-Allocation high-performance networking in Go?**
- Use `sync.Pool` to recycle byte slices and buffers (`bytes.Buffer`), avoiding dynamic heap allocations on every request.

**Q87: What is the difference between Buffered and Unbuffered Channels?**
- Unbuffered: Synchronous rendezvous (sender blocks until receiver is ready).
- Buffered: Asynchronous queue up to capacity $N$ (sender blocks only when buffer is full).

**Q88: What is Starvation vs Deadlock vs Livelock?**
- Deadlock: Two threads wait on each other's locks forever.
- Starvation: A thread is perpetually denied CPU or lock access due to higher-priority threads.
- Livelock: Two threads continuously change states in response to each other without making forward progress.

**Q89: What is CAS (Compare-And-Swap)?**
- Atomic CPU instruction (`CMPXCHG` on x86) that updates a memory location only if it matches an expected old value, forming the foundation of lock-free data structures.

**Q90: What is a Memory Leak in a Garbage-Collected language?**
- Holding references to unused objects in global variables, unbounded caches, or long-lived slices, preventing the GC from reclaiming memory.

**Q91: What is Backpressure and why is it essential in stream processing?**
- Signaling a fast producer to slow down when a consumer's queue is full, preventing Out-Of-Memory (OOM) crashes.

**Q92: What is Split-Brain in distributed systems?**
- A network partition divides a cluster into two isolated sub-clusters, each believing it is the sole active master and writing conflicting data. Mitigated using Quorum ($N/2 + 1$).

**Q93: What is the CAP Theorem?**
- A distributed data store can guarantee at most two of:
  - **C**onsistency: Every read receives the most recent write.
  - **A**vailability: Every non-failing node returns a response.
  - **P**artition Tolerance: System continues operating despite dropped network messages.

**Q94: What is PACELC Theorem?**
- Extends CAP: If there is a **P**artition $\rightarrow$ Trade-off between **A**vailability and **C**onsistency; **E**lse $\rightarrow$ Trade-off between **L**atency and **C**onsistency.

**Q95: What is Tail Latency (p99 / p99.9) and why does it matter more than average latency?**
- In a microservices architecture where 1 page load triggers 100 backend requests, if p99 latency is high, $>63\%$ of end users will experience the slowest request's latency ($1 - 0.99^{100} \approx 0.634$).

**Q96: What is Hedged Requests (Request Hedging)?**
- If a backend request does not respond within p95 time, the load balancer sends a duplicate request to a second backend server and accepts whichever response arrives first, cutting tail latency.

**Q97: What is graceful degradation?**
- When under extreme load, non-essential features (e.g. recommendation widgets, analytics) are disabled to keep core functions (e.g. checkout, payment) operational.

**Q98: What is Little's Law in system capacity planning?**
- $L = \lambda \times W$
- $\text{Concurrent In-Flight Requests} (L) = \text{Arrival Rate} (\lambda) \times \text{Average Latency} (W)$.
- If throughput is 10,000 req/sec and average latency is 50ms (0.05s), the server holds $10,000 \times 0.05 = 500$ concurrent active connections.

**Q99: What is C10K and C1000K problem?**
- The challenge of handling 10,000 (and 1,000,000) concurrent open connections on a single physical server using non-blocking I/O event loops (`epoll`) instead of 1 OS thread per connection.

**Q100: How do you explain this Go Gateway project end-to-end in 60 seconds to a TI interviewer?**
- *"I designed and built a concurrent, high-performance Layer 7 Reverse Proxy and API Gateway in Go. It implements thread-safe Round Robin load balancing using atomic cursors, resilient dual-mode health checking (active background polling coupled with inline closure-based passive failure detection), and connection pooling. The architecture handles failure modes gracefully, isolates backend faults without dropping client requests, and adheres to clean OSI Layer 4/7 separation with zero external heavyweight dependencies."*
