# 06. Round-Robin Implementation and Atomic Operations

## 1. CONCEPT
**Round-Robin** is a scheduling algorithm that selects elements from a list sequentially, wrapping around to the beginning once the end is reached. In a concurrent load balancer, this selection must be fast and thread-safe.

---

## 2. WHY IT EXISTS
Under high load, a reverse proxy receives thousands of requests concurrently.
* If multiple thread goroutines access the selection index at the same time, we have a **critical section**.
* Using a standard index increment (`index++`) is not atomic; it consists of a read, modify, and write CPU instruction cycle. Under concurrency, this causes **race conditions**, leading to lost updates or out-of-bounds errors.
* We must protect the index. We can use a mutex, but **atomic CPU operations** are much faster and more efficient for simple counters.

---

## 3. HOW IT WORKS
We track a counter (`current`) as a 64-bit unsigned integer (`uint64`).
1. Every time a request arrives, we atomically increment the counter.
2. We compute the target backend index using the modulo operator:
   `targetIndex = atomicCounter % totalBackends`
3. Since the counter increases monotonically (1, 2, 3, 4...), the modulo result cycles perfectly (0, 1, 2, 0, 1...).
4. What about integer overflow? When a `uint64` reaches its maximum value ($2^{64}-1$), incrementing it wraps it back to `0` cleanly. Because modulo arithmetic holds under integer wrapping, the distribution remains perfectly correct.

---

## 4. INTERNALS: WHY `index++` IS UNSAFE
A simple `index++` statement compiles down to three assembly-level steps:
1. **Read**: Load the variable from RAM/cache into a CPU register.
2. **Modify**: Increment the value inside the register.
3. **Write**: Store the updated value back to RAM.

If two goroutines (G1 and G2) run concurrently:

```
Goroutine 1 (G1)                        Goroutine 2 (G2)
  │                                       │
  ├─── 1. Read (Index = 5)                │
  │                                       ├─── 1. Read (Index = 5)
  ├─── 2. Increment (Reg = 6)             │
  │                                       ├─── 2. Increment (Reg = 6)
  ├─── 3. Write (Index = 6)               │
  │                                       ├─── 3. Write (Index = 6)
```

Both goroutines incremented the index, but the final value in memory is `6` instead of `7`. One update is lost.
By using **`sync/atomic`**, the CPU locks the memory bus (or L1 cache line) for that specific address during the instruction, guaranteeing that no other CPU core can read or write to it mid-operation.

---

## 5. PROJECT USAGE
In `loadbalancer/roundrobin.go`, we define `current uint64`. In `NextBackend()`, we call `atomic.AddUint64(&bp.current, 1)` and perform the modulo calculation.

---

## 6. CODE WALKTHROUGH
```go
package loadbalancer

import (
	"sync/atomic"
)

type SimplePool struct {
	backends []string
	current  uint64
}

func (sp *SimplePool) Next() string {
	n := uint64(len(sp.backends))
	if n == 0 {
		return ""
	}

	// Atomically increment current and get the new value.
	// This is thread-safe and lock-free.
	val := atomic.AddUint64(&sp.current, 1)

	// Calculate modulo to get index
	idx := val % n
	return sp.backends[idx]
}
```

---

## 7. RUNTIME FLOW
Assume pool size is 2 (Backend A, Backend B). `current` is initially `0`.

```
Client 1 ──► [atomic.AddUint64] ──► current is now 1 ──► 1 % 2 = 1 ──► Backend B
Client 2 ──► [atomic.AddUint64] ──► current is now 2 ──► 2 % 2 = 0 ──► Backend A
Client 3 ──► [atomic.AddUint64] ──► current is now 3 ──► 3 % 2 = 1 ──► Backend B
```

---

## 8. FAILURE CASES
* **Candidate Skipping**: If Backend B is selected but marked unhealthy, we must find another. Simply returning an error breaks availability.
  * *Code Mitigation*: We use a loop inside `NextBackend()` that searches starting from the selected round-robin index, checking up to $N$ times. This ensures we select the next available healthy backend without getting stuck in a loop.

---

## 9. TRADEOFFS
### Atomic Counters vs. Mutexes for Selection
* **Atomic Operations (`sync/atomic`)**:
  * *Pros*: Lock-free concurrency; compiled directly to single CPU instructions (e.g. `LOCK XADD` on x86); order of magnitude faster than mutexes under high contention; avoids thread context switches.
  * *Cons*: Only works on primitive numeric types (integers, pointers); cannot protect complex states or structures.
* **Mutexes (`sync.Mutex`)**:
  * *Pros*: Can protect arbitrary blocks of code, slices, maps, and multiple fields.
  * *Cons*: Higher CPU overhead; blocks goroutines (places them in a wait queue), causing context switches under high load.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: Why is `counter++` not thread-safe in Go?
   * **A**: Because it is not an atomic operation. It consists of three separate steps: loading the value from memory, incrementing it in a register, and writing it back to memory. In a concurrent program, multiple goroutines can execute these steps interleaved, leading to lost updates.
2. **Q**: How does the modulo operator work under integer overflow? Does round-robin break?
   * **A**: It does not break. When an unsigned 64-bit integer overflows, it wraps around to `0` cleanly. For example, if pool size is 3, the value before overflow is $2^{64}-1$. $(2^{64}-1) \pmod 3 = 0$. The next value (after increment) is `0`. $0 \pmod 3 = 0$. Wait, $(2^{64}-1)$ is divisible by 3? Let's check: $2^{64} \pmod 3 = (2 \pmod 3)^{64} = (-1)^{64} = 1$. Thus, $(2^{64}-1) \pmod 3 = 0$. Since both $(2^{64}-1) \pmod 3$ and $0 \pmod 3$ evaluate to `0`, the transition is completely seamless.
3. **Q**: What does the CPU do at the hardware level during an atomic operation?
   * **A**: The CPU core executes a specialized instruction prefix (like `LOCK` in x86). This locks the cache line containing the variable, preventing other cores from accessing it through their L1/L2 caches or the main memory bus until the instruction completes.
