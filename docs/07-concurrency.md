# 07. Go Concurrency Model and Schedulers

## 1. CONCEPT
Go features first-class support for concurrency through **Goroutines** and **Channels**. Concurrency is the composition of independently executing processes. In our load balancer, concurrency allows us to handle thousands of requests simultaneously while running active background health checks.

---

## 2. WHY IT EXISTS
Traditional network servers scale concurrency using OS threads or processes.
* **OS Threads** are heavy. They have fixed-size memory stacks (typically 1MB to 8MB) and context-switching between them requires entering the OS kernel, which is slow.
* To achieve high throughput (HPC, Cloud scale), a server must handle connections with minimal memory and CPU context-switch overhead.
* Go introduces goroutines to solve this, allowing applications to run hundreds of thousands of concurrent tasks on a small number of physical cores.

---

## 3. HOW IT WORKS
### Goroutines vs. OS Threads
| Feature | Goroutine | OS Thread |
| :--- | :--- | :--- |
| **Memory footprint** | Starts at ~2KB (dynamic) | 1MB - 8MB (fixed) |
| **Creation cost** | Extremely cheap (user-space allocation) | Expensive (system call allocation) |
| **Context Switch** | Fast (~10-100ns, user-space) | Slow (~1000-2000ns, kernel transition) |
| **Managed by** | Go Runtime Scheduler | Operating System Kernel |

---

## 4. INTERNALS: THE G-M-P SCHEDULER
Go manages concurrency using the **G-M-P model** within its runtime scheduler:

```
    [ G ]  (Goroutine: represents the code/task)
      │
      ▼
    [ P ]  (Processor: logical resource, GOMAXPROCS count)
      │
      ▼
    [ M ]  (Machine: physical OS thread created by OS kernel)
      │
      ▼
   [ CPU Core ]
```

* **G (Goroutine)**: Represents the goroutine. It contains the execution stack, instruction pointer, and scheduling state.
* **M (Machine/Thread)**: Represents a physical OS thread.
* **P (Processor)**: Represents a logical processor or context required to execute Go code. The number of P's matches `runtime.GOMAXPROCS` (defaults to the number of logical CPU cores).

### Work Stealing and Syscall Hand-off
* **Local Run Queue**: Each `P` holds a queue of runnable goroutines `G`.
* **Work Stealing**: If a `P` runs out of work, it steals half the goroutines from another `P`'s queue.
* **Syscall Block (Hand-off)**: If a running goroutine `G` makes a blocking system call (like reading a file), the scheduler detaches the physical thread `M` from `P`. `P` then runs other goroutines on a *new* or idle thread `M`. Once the syscall completes, `G` is placed back onto an available `P`'s queue.

---

## 5. PROJECT USAGE
We utilize concurrency in two places:
1. **Per-Request Goroutines**: Every incoming request to our gateway triggers `ServeHTTP` inside a goroutine spawned by `net/http`.
2. **Background Health Polling**: The `HealthChecker` starts a background goroutine loop to run periodic checks. It spawns nested concurrent goroutines to query downstream backends in parallel.

---

## 6. CODE WALKTHROUGH
Parallel health checking inside `health/checker.go`:

```go
package health

import (
	"sync"
)

type DummyBackend struct {
	Addr string
}

func CheckAllBackendsConcurrently(backends []DummyBackend) {
	var wg sync.WaitGroup

	for _, b := range backends {
		wg.Add(1) // Increment the waitgroup counter

		// Spawn a concurrent goroutine for each backend check
		go func(addr string) {
			defer wg.Done() // Decrement counter when goroutine exits
			pingBackend(addr)
		}(b.Addr)
	}

	// Blocks main routine until all check goroutines call wg.Done()
	wg.Wait() 
}

func pingBackend(addr string) {}
```

---

## 7. RUNTIME FLOW
```
Main HTTP Server Goroutine
         │
         ├─── (spawn) ──► Goroutine: Health Check Ticker Loop
         │                  │
         │                  ├─── (spawn) ──► Goroutine: Ping Backend A
         │                  ├─── (spawn) ──► Goroutine: Ping Backend B
         │                  ▼
         │                Wait for all pings (sync.WaitGroup)
         │
  (Incoming Client Connections)
         ├─── (spawn) ──► Goroutine: Serve Client 1 (HTTP Request)
         └─── (spawn) ──► Goroutine: Serve Client 2 (HTTP Request)
```

---

## 8. FAILURE CASES
* **Goroutine Leak**: If a goroutine is blocked waiting on a channel read or network call without a timeout, it remains in memory forever. Under high load, leaking goroutines will exhaust server RAM.
  * *Code Mitigation*: In `HealthChecker.Stop()`, we close the stop channel and use `sync.WaitGroup` to block shutdown until the background poller exits, ensuring clean lifecycle management.

---

## 9. TRADEOFFS
### Concurrency vs. Parallelism
* **Concurrency**: Structure code as multiple independent tasks executing asynchronously. Can run on a single CPU core via time-slicing.
* **Parallelism**: Run multiple tasks at the exact same physical instant on multi-core hardware.
* *Go's model* handles both: it allows you to write highly concurrent code that automatically scales to run in parallel as you add more CPU cores.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: What is the difference between a Goroutine and an OS Thread?
   * **A**: A goroutine is a user-space thread managed by the Go runtime, starting with a tiny 2KB dynamic stack. OS threads are managed by the kernel, requiring a fixed 1MB-8MB stack. Goroutine context switches are faster because they don't require transitioning into kernel space.
2. **Q**: Describe the G-M-P scheduler model in Go.
   * **A**: **G** represents a goroutine, **M** represents a physical OS thread, and **P** represents a logical processor context. The scheduler maps `G`s onto `M`s using `P`s. It maximizes CPU utilization using work-stealing and syscall hand-offs.
3. **Q**: What is a Goroutine Leak, and how can you prevent it?
   * **A**: A goroutine leak occurs when a goroutine is launched but can never terminate, usually because it is blocked indefinitely waiting on a channel or resource. It is prevented by always ensuring channels have send/receive completions, using context timeouts on blocking calls, and coordinating lifecycles via cancellation channels.
