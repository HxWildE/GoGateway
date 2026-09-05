# System Design: Rate Limiter (HLD + LLD + Algorithm Deep Dive)

> ⚠️ **IMPORTANT NOTE**: Rate Limiting is **NOT** currently implemented in the GoGateway codebase. This document is provided purely for conceptual understanding and system design interview preparation.

> **Goal**: Interviewer ko Rate Limiting ke 5 standard algorithms, distributed race conditions, Redis Lua scripts, aur HTTP headers aise samjhana ki low-level architecture crystal clear ho jaye.

---

## 1. Rate Limiter kya hai aur kyu zaroori hai?

Rate limiter ek component hai jo check karta hai ki koi particular client (IP, User ID, API Key) ek specific time window me kitni requests bhej raha hai. Agar limit exceed ho jaye, toh excess requests ko reject kar deta hai (`HTTP 429 Too Many Requests`).

### Why do we need it?
1. **DDoS & Brute-Force Attack Prevention**: Login endpoint par 1 second me 10,000 passwords guess karne se rokna.
2. **Cost Control**: Third-party paid APIs (e.g. OpenAI, Stripe, Twilio) par billing burst hone se bachana.
3. **Prevent Resource Starvation (Noisy Neighbor)**: Ek greedy user pura backend CPU/DB choke na kar de.
4. **Traffic Shaping**: Sudden traffic spikes ko smooth stream me convert karna.

---

## 2. The 5 Core Rate Limiter Algorithms (With Math & Code Logic)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           5 RATE LIMITING ALGORITHMS                        │
├──────────────────────┬──────────────────────┬───────────────────────────────┤
│ 1. Token Bucket      │ 2. Leaky Bucket      │ 3. Fixed Window Counter       │
│ (Bursty traffic OK)  │ (Smooth constant out)│ (Simple, but boundary bug)    │
├──────────────────────┴──────────────────────┴───────────────────────────────┤
│ 4. Sliding Window Log (Exact, Memory heavy)                                 │
│ 5. Sliding Window Counter (Hybrid, Memory efficient, Recommended Industry)  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Algorithm 1: Token Bucket (Most Popular - AWS, Stripe, Google)

#### Concept:
- Ek bucket hai jiski capacity $C$ tokens hai.
- Bucket me continuously ek fixed rate $R$ tokens/sec se tokens girte hain.
- Jab bhi request aati hai:
  - Agar token available hai: 1 token consume hota hai aur request allow hoti hai.
  - Agar bucket empty hai: Request drop (`429`) ho jati hai.

#### 💡 The Mathematical Trick (No Background Tickers!):
Log sochte hain ki background me ek thread/timer lagana padega token add karne ke liye. **WRONG!**
Background timer millions of users ke liye scale nahi karta (CPU thrashing).

**Formula based Lazy Refill on Request Arrival**:
$$\text{Tokens to Add} = (\text{CurrentTime} - \text{LastRefillTime}) \times \text{RefillRate}$$
$$\text{CurrentTokens} = \min(\text{Capacity}, \text{CurrentTokens} + \text{Tokens to Add})$$

```go
type TokenBucket struct {
    capacity     int64
    tokens       float64
    refillRate   float64 // tokens per second
    lastRefill   time.Time
    mu           sync.Mutex
}

func (tb *TokenBucket) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    now := time.Now()
    elapsed := now.Sub(tb.lastRefill).Seconds()
    tb.lastRefill = now

    // Refill tokens based on elapsed time
    tb.tokens = math.Min(float64(tb.capacity), tb.tokens + elapsed * tb.refillRate)

    if tb.tokens >= 1.0 {
        tb.tokens -= 1.0
        return true // ALLOWED
    }
    return false // REJECTED (HTTP 429)
}
```

- **Pros**: Bursts allow karta hai (jaise bucket full hai toh $C$ requests instant pass ho sakti hain). Memory efficient ($O(1)$ per user).
- **Cons**: Bursts can temporarily stress backend.

---

### Algorithm 2: Leaky Bucket (Traffic Shaping - NGINX)

#### Concept:
- Ek FIFO Queue (Bucket) hoti hai jisme requests aati hain.
- Bucket ke neeche ek hole hai jisse requests ek **constant rate** par process hoti hain (leak hoti hain).
- Agar incoming rate leak rate se zyada ho aur bucket (queue) full ho jaye, toh new requests overflow (drop) ho jati hain.

```
       Incoming Requests (Bursty) ──►  │ █ █ █ █ │  (FIFO Queue)
                                       │ █ █ █   │
                                       └───┬───┘
                                           │ Leak at constant rate (e.g. 5 req/sec)
                                           ▼
                                    Backend Service
```

- **Pros**: Output traffic bilkul smooth constant rate ka hota hai.
- **Cons**: Sudden legitimate burst ke time new requests drop ho sakti hain chahe backend idle ho.

---

### Algorithm 3: Fixed Window Counter

#### Concept:
- Timeline ko fixed windows me divide karte hain (e.g., 1:00-1:01, 1:01-1:02).
- Har window me counter hota hai. Agar limit 100 req/min hai aur window counter $>100$, reject.

#### ⚠️ Critical Flaw: The 2x Boundary Burst Bug
Interviewer zaroor puchega: *"Fixed window me kya problem hai?"*
- Maan lo limit hai 100 req/min.
- User ne 1:00:59 par 100 requests bheji (Window 1 allows it).
- User ne agle hi second 1:01:01 par fir se 100 requests bheji (Window 2 allows it).
- **Reality**: 2 second ke interval (1:00:59 se 1:01:01) me **200 requests pass ho gayi** (2x overload)! Server crash!

---

### Algorithm 4: Sliding Window Log

#### Concept:
- Har request ka exact timestamp store karte hain (e.g. Redis Sorted Set `ZSET`).
- Jab new request aati hai:
  1. $t - \text{window\_size}$ se purane sabhi timestamps delete karo (`ZREMRANGEBYSCORE`).
  2. Set ki current size check karo (`ZCARD`).
  3. Agar size $<$ limit, new timestamp add karo (`ZADD`) aur allow karo. Otherwise reject.

- **Pros**: 100% accurate sliding window. Boundary bug impossible.
- **Cons**: **Massive Memory Footprint**! Agar 10,000 req/min limit hai toh 10,000 timestamp entries memory me rakhni padengi har active user ke liye.

---

### Algorithm 5: Sliding Window Counter (Industry Standard Approximation)

#### Concept:
Fixed Window ki memory efficiency + Sliding Window Log ki accuracy ka hybrid!
- Formula:
$$\text{Estimated Requests} = \text{Current Window Count} + \left(\text{Previous Window Count} \times \frac{\text{Window Size} - \text{Time into Current Window}}{\text{Window Size}}\right)$$

```
Previous Window (Count: 80)          Current Window (Count: 30)
[-----------------------------------|--------*--------------------------]
                                    1:00     1:18 (30% through window)
                                    
Estimated = 30 + (80 * 0.70) = 30 + 56 = 86 requests in sliding 60s!
```

- Agar limit 100 hai aur $86 < 100$, allow request!
- **Pros**: Super low memory (sirf 2 counters per user). Very accurate (error rate $<0.05\%$).

---

## 3. Distributed Rate Limiter & Concurrency (Redis + Lua Script)

Jab tumhare paas 10 API Gateway instances honge, toh local memory me rate limit rakhne se kaam nahi chalega (User Gateway 1 par 10 req bhejega, Gateway 2 par 10 bhejega $\rightarrow$ 100 req cross ho jayengi).

### Centralized Store: Redis
Sabhi Gateways Redis se rate check karenge.

### ⚠️ Race Condition in Redis (Check-Then-Set Bug):
```
Gateway 1: GET user:123:count  (returns 99)
Gateway 2: GET user:123:count  (returns 99)
Gateway 1: INCR user:123:count (sets 100) -> ALLOWED
Gateway 2: INCR user:123:count (sets 101) -> ALLOWED (BURST BUG! Limit exceeded)
```

### ✅ Solution: Redis Lua Script (Atomic Execution)
Redis single-threaded hota hai aur Lua script ko atomically run karta hai bina kisi intermediate interrupt ke.

```lua
-- KEYS[1]: user key (e.g. "rate:user123")
-- ARGV[1]: limit (e.g. 100)
-- ARGV[2]: window_seconds (e.g. 60)

local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[2])
end

if current > tonumber(ARGV[1]) then
    return 0 -- REJECTED
else
    return 1 -- ALLOWED
end
```

---

## 4. Rate Limiter Standard HTTP Headers (RFC Standards)

Jab Rate Limiter kisi client ko response deta hai, standard HTTP headers return hone chahiye:

| HTTP Header | Example Value | Description |
| :--- | :--- | :--- |
| `X-RateLimit-Limit` | `100` | Current window me maximum allowed requests |
| `X-RateLimit-Remaining` | `4` | Window khatam hone tak kitni requests bachi hain |
| `X-RateLimit-Reset` | `1725062400` | Unix timestamp jab bucket/window reset hogi |
| `Retry-After` | `12` | (Returned on 429) Client ko kitne seconds baad retry karna chahiye |
| **HTTP Status** | `429 Too Many Requests` | Standard rejection status code |
