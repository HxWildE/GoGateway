# 05. Rate Limiting Algorithms & Distributed Architecture

```
========================================================================================
                          5 RATE LIMITER ALGORITHMS COMPARED
========================================================================================

1. TOKEN BUCKET (Bursts Allowed, Lazy Refill)
   Refill Rate: 5 tokens/sec
   ┌───────────────────────┐
   │ 🪙  🪙  🪙  🪙  🪙    │  Capacity: 10
   │ 🪙  🪙  🪙            │
   └───────────┬───────────┘
               │ 1 token consumed per request
               ▼
   [ Passed: HTTP 200 OK ]   (Empty? -> [ Rejected: HTTP 429 ])


2. LEAKY BUCKET (Constant Output Rate / Traffic Shaping)
   Bursty Inflow: 100 reqs/sec
               ▼
   ┌───────────────────────┐
   │ 💧 💧 💧 💧 💧 💧 💧  │  FIFO Queue Buffer (Capacity 50)
   │ 💧 💧 💧              │  (Overflow drops immediately)
   └───────────┬───────────┘
               │ Constant leak: 5 reqs/sec
               ▼
   [ Steady Stream to Backend ]


3. FIXED WINDOW (Has 2x Boundary Spike Bug)
   Window 1 (1:00-1:01): [ 100 requests at 1:00:59 ] -> PASS
   Window 2 (1:01-1:02): [ 100 requests at 1:01:01 ] -> PASS
   ❌ 200 requests passed in a 2-second interval!


4. SLIDING WINDOW LOG (Exact, Memory Heavy)
   User Sorted Set: [ 1700000001, 1700000003, 1700000012, 1700000059 ]
   Step 1: Delete all entries < (now - 60s)
   Step 2: Check count of remaining timestamps
   Step 3: If count < Limit, add current timestamp


5. SLIDING WINDOW COUNTER (Hybrid Standard: 2 Integers, <0.05% Error)
   [ Previous Window: 80 reqs ]  |  [ Current Window (30% elapsed): 30 reqs ]
   
   Estimated = 30 + (80 * (1.0 - 0.30)) = 30 + 56 = 86 reqs in sliding 60s.
```

---

## 🌐 Distributed Rate Limiting with Redis & Lua Script

```mermaid
flowchart TD
    subgraph Gateways ["Multi-Node API Gateway Layer"]
        GW1["Gateway Node 1"]
        GW2["Gateway Node 2"]
        GW3["Gateway Node 3"]
    end

    subgraph RedisCluster ["Centralized Redis Cluster"]
        Lua["⚡ Atomic Lua Script<br/>EVALSHA script 1 key 100 60"]
        HashStore["Key: rate:user_123<br/>Count: 84<br/>TTL: 26s"]
    end

    GW1 -->|Atomic EVAL| Lua
    GW2 -->|Atomic EVAL| Lua
    GW3 -->|Atomic EVAL| Lua
    Lua <--> HashStore
    Lua -->|Allowed: return 1| GW1
    Lua -->|Blocked: return 0| GW2
    GW2 -->|HTTP 429 Too Many Requests| DropClient["📱 Client Rejected"]
```
