# 15. System Design and Interview Preparation Bank

This document acts as your comprehensive preparation manual for technical interviews (specifically tailored for hardware-software boundaries and system-level roles like those at Texas Instruments). It outlines the **Interviewer Attack Path** and categorizes high-probability questions.

---

## 1. THE INTERVIEWER ATTACK PATH
This represents the natural trajectory of a system-level interview. An interviewer will start with high-level architecture and systematically drill down into operating system kernel boundaries, protocols, and concurrency mechanics.

```
          [ High-Level Design ]  "What did you build?"
                   │
                   ▼
         [ Architectural Choice ] "Why a reverse proxy? How does it differ from a load balancer?"
                   │
                   ▼
         [ Protocol Breakdown ]   "What happens to the HTTP request bytes under the hood?"
                   │
                   ▼
         [ Transport Deep-Dive ]  "How does the underlying TCP connection establish and behave?"
                   │
                   ▼
         [ Concurrency & OS ]     "How does Go execute multiple requests? Goroutine vs Thread?"
                   │
                   ▼
         [ Shared State & Race ]  "How did you protect the backend pool states? Mutex details?"
                   │
                   ▼
         [ Network Failures ]     "What if a backend dies mid-request? Active vs Passive recovery?"
                   │
                   ▼
         [ Systems Scaling ]      "How would you scale this to millions of requests in production?"
```

---

## 2. INTERVIEW QUESTION BANK BY LEVELS

### LEVEL 1 — PROJECT BASICS

#### Q1: What did you build, and what are its key components?
* **Expected Answer**: I built a concurrent HTTP Reverse Proxy and Load Balancer in Go. It consists of: a Gateway HTTP Server that handles OS signals for graceful shutdown; a Backend Pool that executes an atomic index-based Round-Robin selection; a Reverse Proxy wrapper with customized headers and timeout-configured transports; and a concurrent active background Health Checker.
* **Reasoning**: Proves you understand the high-level boundaries of your own codebase.
* **Common Mistake**: Ramble without highlighting clear component boundaries.
* **Possible Follow-up**: "Why did you build your own reverse proxy instead of using NGINX?"
  * *Answer*: "To understand networking and systems fundamentals—specifically connection pooling, active/passive failure states, and thread-safe data structures—which are often hidden behind NGINX configurations."

---

### LEVEL 2 — GO LANDSCAPE

#### Q2: What is Go's escape analysis, and how does it affect memory?
* **Expected Answer**: Escape analysis is a compile-time process where the compiler decides whether a variable can be allocated on the function's stack frame or if it must 'escape' to the heap. If a variable is referenced outside the stack frame (like returning a pointer), it escapes. Stack allocations are fast and self-cleaning, while heap allocations require garbage collection.
* **Reasoning**: Demonstrates language runtime internals.
* **Common Mistake**: Claiming that stack vs. heap allocation is determined at runtime or by using `new()`.
* **Possible Follow-up**: "Does returning a pointer from a constructor function always cause a heap allocation?"
  * *Answer*: "Yes, because the caller needs to access the variable after the constructor's stack frame is popped."

---

### LEVEL 3 — HTTP

#### Q3: Why did you preserve or modify headers like `X-Forwarded-Host` and `X-Forwarded-Proto`?
* **Expected Answer**: Because the proxy establishes a direct TCP connection with downstream backends. To the backend, the request host is the proxy's IP/port, and the schema is HTTP. We inject `X-Forwarded-Host` (original host requested by the client) and `X-Forwarded-Proto` (HTTP or HTTPS) so the backend can generate correct absolute redirects, compile audit trails, and apply security rules.
* **Reasoning**: Shows understanding of application-layer protocol translation.
* **Common Mistake**: Confusing `Host` with `X-Forwarded-Host`.
* **Possible Follow-up**: "What is the security risk of trusting `X-Forwarded-For` blindly?"
  * *Answer*: "IP spoofing. A client can send a fake `X-Forwarded-For` header. The proxy should append the client IP, or only trust headers from trusted upstream proxies."

---

### LEVEL 4 — TCP & NETWORKING

#### Q4: Walk through the TCP 3-way handshake and describe the socket state transitions.
* **Expected Answer**: The client socket transitions to `SYN-SENT` and sends a `SYN` packet. The server (in `LISTEN` state) receives it, transitions to `SYN-RECEIVED`, and sends a `SYN-ACK`. The client receives it, transitions to `ESTABLISHED`, and replies with `ACK`. Once the server receives the `ACK`, its socket transitions to `ESTABLISHED`.
* **Reasoning**: Standard networking core question (crucial for TI).
* **Common Mistake**: Mixing up the order of SYN and ACK.
* **Possible Follow-up**: "What is a SYN flood attack, and how do operating systems mitigate it?"
  * *Answer*: "An attacker floods a server with SYN packets but never sends the final ACKs, filling the server's backlog queue. OS kernels mitigate this using **SYN Cookies**, where connection details are encoded inside the SYN-ACK sequence number instead of holding memory state early."

---

### LEVEL 5 — CONCURRENCY

#### Q5: Explain the difference between Goroutines and OS Threads.
* **Expected Answer**: Goroutines are managed in user-space by the Go runtime; they start with a 2KB stack that grows dynamically. OS threads are managed by the kernel, requiring a fixed 1MB-8MB stack. Goroutine context switches are faster (~10-100ns) because they avoid entering kernel space, whereas thread switches (~1-2µs) require register saves, page table switches, and kernel transitions.
* **Reasoning**: Systems-level understanding of thread scheduling.
* **Common Mistake**: Saying goroutines are threads.
* **Possible Follow-up**: "What happens when a goroutine performs a blocking system call (like reading from a socket)?"
  * *Answer*: "The Go scheduler executes a Hand-off. It detaches the OS thread `M` from its logical processor `P`. `P` then runs other goroutines on a different thread, while `M` blocks in the kernel waiting for the syscall to complete, preventing the entire application from stalling."

---

### LEVEL 6 — FAILURE HANDLING

#### Q6: How does your load balancer handle a slow backend, and what is passive failure detection?
* **Expected Answer**: We set timeouts on our proxy Transport `DialContext` so requests don't hang. Passive failure detection is traffic-driven: if a client request routed through our proxy fails (e.g. connection refused), the proxy's `ErrorHandler` executes a callback to mark that backend dead immediately (`SetAlive(false)`), without waiting for the next active background health check poll.
* **Reasoning**: Demonstrates robust architectural edge-case engineering.
* **Common Mistake**: Suggesting that we let requests hang until the active health check detects the failure.
* **Possible Follow-up**: "What is the danger of passive failure detection under high traffic?"
  * *Answer*: "If there is a brief network blip, many concurrent request goroutines might fail simultaneously, triggering the callback repeatedly. We must protect the status update with a mutex to avoid race conditions, and potentially implement thresholds to avoid thrashing."

---

### LEVEL 7 — SYSTEM DESIGN

#### Q7: How would you scale this load balancer to handle millions of concurrent connections?
* **Expected Answer**: 
  1. **Horizontal Scaling**: Deploy multiple instances of our gateway.
  2. **DNS Load Balancing / Anycast**: Use Geo-DNS or Anycast routing to distribute client connections across different gateway instances.
  3. **L4 Load Balancer**: Place a Layer-4 load balancer (like IPVS or HAProxy running L4 TCP routing) in front of our L7 proxies to distribute TCP connection pipes.
  4. **Kernel Tuning**: Optimize OS limits (e.g., maximum open file descriptors `ulimit -n` and local port range `/proc/sys/net/ipv4/ip_local_port_range`).
* **Reasoning**: Demonstrates knowledge of enterprise scale.
* **Common Mistake**: Claiming a single instance of our Go server can handle millions of connections without kernel or horizontal scaling optimizations.

---

### LEVEL 8 — DEEP DIVE / TRICK QUESTIONS

#### Q8: Why did you use `sync/atomic` for round-robin instead of a Mutex? Is atomic always faster?
* **Expected Answer**: I used `sync/atomic` because updating a simple index counter is a primitive numeric operation. Atomics translate to single CPU-level lock-free instructions (like `LOCK XADD` on x86), avoiding the overhead of goroutine blocking, scheduling queues, and context switches associated with Mutexes. However, atomics are only faster for simple numeric types; if we need to update multiple fields or slices cohesively, we *must* use a Mutex.
* **Reasoning**: Deep systems engineering optimization question.
* **Common Mistake**: Claiming that atomic operations have zero CPU overhead.
* **Possible Follow-up**: "What is False Sharing, and how does it relate to atomic counters?"
  * *Answer*: "False sharing occurs when variables accessed by different CPU cores sit on the same cache line (usually 64 bytes). When one core writes to its variable atomically, it invalidates the L1/L2 cache line of the other core, causing CPU stall cycles. We can mitigate this using cache-line alignment padding."
