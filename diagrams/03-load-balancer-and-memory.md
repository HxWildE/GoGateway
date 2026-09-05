# 03. Load Balancer, Atomic Operations & Memory Layout

```
========================================================================================
                      ROUND ROBIN ATOMIC CURSOR & MEMORY MAP
========================================================================================

                  loadbalancer.BackendPool in RAM
   ┌─────────────────────────────────────────────────────────────┐
   │  mu      : sync.RWMutex (24 bytes - Reader/Writer lock)     │
   │  cursor  : uint64 (8 bytes - Monotonically increasing)      │
   │  backends: []*Backend (24 bytes slice header: ptr, len, cap)│
   └──────────────────────────────┬──────────────────────────────┘
                                  │
                                  │ Slice elements point to Heap
        ┌─────────────────────────┼─────────────────────────┐
        ▼                         ▼                         ▼
  ┌───────────┐             ┌───────────┐             ┌───────────┐
  │ Backend 0 │             │ Backend 1 │             │ Backend 2 │
  │ (:8081)   │             │ (:8082)   │             │ (:8083)   │
  │ Alive:true│             │ Alive:true│             │ Alive:false
  └───────────┘             └───────────┘             └───────────┘
```

---

## 🔒 Lock-Free Atomic Cursor Math (Why Mutex is Avoided)

```
                       Request Arrival Sequence
                  
   Goroutine A (Req 1) ────► atomic.AddUint64(&cursor, 1) ──► returns 1
                             Index = (1 - 1) % 3 = 0  ──► Backend 0 (:8081)
                             
   Goroutine B (Req 2) ────► atomic.AddUint64(&cursor, 1) ──► returns 2
                             Index = (2 - 1) % 3 = 1  ──► Backend 1 (:8082)
                             
   Goroutine C (Req 3) ────► atomic.AddUint64(&cursor, 1) ──► returns 3
                             Index = (3 - 1) % 3 = 2  ──► Backend 2 (:8083)
                             
   Goroutine D (Req 4) ────► atomic.AddUint64(&cursor, 1) ──► returns 4
                             Index = (4 - 1) % 3 = 0  ──► Backend 0 (:8081)
```

```
⚡ Low-Level CPU Instruction:
x86 Assembly: LOCK XADD [cursor], 1
No OS context switch! Executed directly in CPU L1 cache pipeline in ~1 nanosecond.
```

---

## 🎯 Consistent Hashing Ring (Distributed LB Alternative)

```
                           THE HASH RING (0 to 2^32 - 1)
                               
                                  0 / 2^32
                                     ▲
                                     │
                 [Backend C-vnode1]  │   [Backend A-vnode1]
                        \            │            /
                         \           │           /
           (hash: 3.2B)   \          │          /  (hash: 0.8B)
                           \         │         /
                            \        │        /
                             \       │       /
   [Backend B-vnode2] ────────┼───────┼───────┼──────── [Backend B-vnode1]
   (hash: 2.8B)               │       │       │         (hash: 1.5B)
                               \      │      /
    Key: hash("user_459") ───►  \     │     /   ──► Clockwise lookup hits:
    (hash: 2.1B)                 \    │    /        [Backend B-vnode2]
                                  \   │   /
                                   \  │  /
                                    \ │ /
                             [Backend A-vnode2]
                                (hash: 2.4B)
```

```
Key Migration on Node Crash:
- Modulo Hash (K % N) : Remaps ~100% of keys (Disaster for Caches)
- Consistent Hash Ring: Remaps only 1/N fraction of keys (Graceful)
```
