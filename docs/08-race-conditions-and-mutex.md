# 08. Race Conditions, Mutexes, and Read-Write Locks

## 1. CONCEPT
A **Race Condition** occurs when two or more goroutines access shared memory concurrently, at least one access is a write, and there is no synchronization to order the accesses. To prevent race conditions, we use **Mutual Exclusion (Mutexes)** or atomic operations.

---

## 2. WHY IT EXISTS
In a high-performance network server, state is shared:
* Client requests (routed concurrently in separate goroutines) read the health status of backends (`IsAlive()`).
* The background health checker (running in its own goroutine) writes the health status of backends (`SetAlive()`).
* If G1 reads a memory location while G2 is modifying it, the read can observe corrupt or stale data. In Go, concurrent writes to complex structures (like maps or slices) can cause the program to crash immediately.

---

## 3. HOW IT WORKS
### Mutex (`sync.Mutex`)
A Mutex provides exclusive access. Only one goroutine can hold the lock at any time. Any other goroutine trying to acquire the lock will block until the owner releases it.

```
Goroutine A               Mutex                Goroutine B
    │                       │                       │
    ├─── Lock() ───────────►│ (Locked)              │
    │                       │                       ├─── Lock() (Blocks...)
    ├─── Write State ───────│                       │    .
    └─── Unlock() ─────────►│ (Released)            │    .
                            │◄─── Grabs Lock ───────┤
                            │     (Locked)          │
                            │     Write State ──────┤
                            │◄─── Unlock() ─────────┘
```

### Read-Write Mutex (`sync.RWMutex`)
A Read-Write Mutex is a specialized lock that allows multiple concurrent readers, but only a single writer:
* **RLock()**: Multiple goroutines can read the data simultaneously if no write lock is held.
* **Lock()**: Only a single writer can modify the data. All readers and other writers are blocked.
This provides a huge performance gain for structures that are read frequently but updated rarely.

---

## 4. INTERNALS: MUTEX STATE MACHINE
A Go `sync.Mutex` is represented internally by a single 32-bit integer state and a semaphore. The state tracks:
* **Locked Bit**: Is the lock active?
* **Woken Bit**: Has a waiting goroutine been woken up?
* **Starvation Mode**: Under heavy lock contention, waiting goroutines can starve. If a goroutine fails to acquire the lock for more than 1 millisecond, the Mutex transitions into **Starvation Mode**. In this mode, releasing the lock hands ownership directly to the head of the wait queue, bypassing newly arriving goroutines.

---

## 5. PROJECT USAGE
In `loadbalancer/roundrobin.go`, we use `sync.RWMutex` on the `Backend` struct:
* `IsAlive()` uses `RLock()` and `RUnlock()` because reading health status happens on every client request.
* `SetAlive()` uses `Lock()` and `Unlock()` because updating health status happens only periodically.

---

## 6. CODE WALKTHROUGH
```go
package loadbalancer

import (
	"sync"
)

type ThreadSafeState struct {
	mu    sync.RWMutex
	value int
}

func (s *ThreadSafeState) Read() int {
	s.mu.RLock()         // Multiple readers can acquire RLock simultaneously
	defer s.mu.RUnlock() // Ensures unlock executes even if panic occurs
	return s.value
}

func (s *ThreadSafeState) Write(v int) {
	s.mu.Lock()         // Exclusive lock; blocks readers and other writers
	defer s.mu.Unlock()
	s.value = v
}
```

---

## 7. RUNTIME FLOW
```
Client 1 (Read)  ──► RLock()  ──► [Granted]
Client 2 (Read)  ──► RLock()  ──► [Granted]  (Concurrent reads ok!)
HealthCheck(Write)──► Lock()   ──► [Blocked]  (Waits for Client 1 & 2 to RUnlock)
Client 1 & 2     ──► RUnlock()
HealthCheck      ──► Lock()   ──► [Granted]  (All reads now blocked)
HealthCheck      ──► Unlock()
```

---

## 8. FAILURE CASES
* **Deadlock**: Occurs when two or more goroutines are blocked indefinitely, each waiting for a lock held by the other.
  * *Code Mitigation*: Always acquire locks in the same order throughout the codebase, keep critical sections small, and use `defer` to unlock immediately.
* **Copying a Mutex**: A Mutex contains internal state tracking. If you copy a struct containing a Mutex (e.g. by value assignment), the lock state is copied, leading to undefined locking behavior.
  * *Mitigation*: Always pass structs containing mutexes by **pointer** (`*Backend`), never by value.

---

## 9. TRADEOFFS
### Mutex (`sync.Mutex`) vs. Read-Write Mutex (`sync.RWMutex`)
* **`sync.Mutex`**:
  * *Pros*: Simpler code; slightly lower CPU overhead to acquire/release (only checks a single bit).
  * *Cons*: Blocks readers during other reads; reduces throughput if read-to-write ratio is high.
* **`sync.RWMutex`**:
  * *Pros*: High read concurrency; readers do not block other readers.
  * *Cons*: Marginally higher internal overhead; can lead to writer starvation if readers continuously hold lock (mitigated by Go's internal starvation logic).

---

## 10. INTERVIEW QUESTIONS
1. **Q**: What is a Race Condition? How do you detect them in Go?
   * **A**: A race condition occurs when concurrent goroutines access the same memory address without synchronization, and at least one access is a write. We detect them using Go's built-in race detector by running tests or binaries with the `-race` flag (e.g., `go test -race ./...`).
2. **Q**: What is the difference between `sync.Mutex` and `sync.RWMutex`?
   * **A**: A `sync.Mutex` is an exclusive lock. Only one goroutine can access the critical section. A `sync.RWMutex` allows multiple concurrent readers (`RLock`), but only a single exclusive writer (`Lock`), optimizing performance for read-heavy workloads.
3. **Q**: What is Starvation Mode in Go's Mutex implementation?
   * **A**: Under high contention, newly spawned goroutines can acquire the lock because they are already hot on the CPU, while queued goroutines that just woke up have to context-switch, losing the race. Starvation Mode shifts the Mutex behavior so that when a lock is released, ownership is passed directly to the head of the wait queue, preventing starvation.
4. **Q**: Why is it dangerous to copy a Mutex?
   * **A**: A Mutex contains private fields tracking lock status and waiters. Copying a Mutex duplicates this state. If the original was locked, the copy will start locked, but unlocking the copy will not release the original, leading to deadlocks and corrupt synchronization state.
