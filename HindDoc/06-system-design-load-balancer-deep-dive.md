# System Design: Load Balancer (HLD + LLD + Computer Networks Deep Dive)

> **Goal**: Interviewer ke saamne Load Balancer ka ek-ek layer (L4 vs L7, Sockets, Consistent Hashing, Health Checking, Race conditions) aise explain karna ki usse lage tumne Linux Kernel aur Network Stack scratch se likha hai.

---

## 1. Load Balancer kya hai? (Core Concept & Why It Exists)

Imagine karo ek famous restaurant hai jisme 1 cashier/chef hai. 1000 log ek saath khana lene aa gaye. 
- Chef ka CPU 100% ho jayega, requests drop hongi, restaurant crash ho jayega.
- **Solution**: 5 chef rakho aur gate par ek **Traffic Manager (Load Balancer)** khada karo jo decide karega kaunsa customer kis chef ke paas jayega.

```
       [ Client / Browser ] 
                │
                ▼  (Public IP: 198.51.100.1)
     ┌──────────────────────┐
     │    LOAD BALANCER     │  <--- Single Point of Ingress
     │ (Reverse Proxy / LB) │
     └──────────┬───────────┘
                │  (Private Network / VPC)
       ┌────────┼────────┐
       ▼        ▼        ▼
   [Server 1] [Server 2] [Server 3]
```

### Key Responsibilities:
1. **High Availability (HA)**: Ek server mar gaya toh traffic doosre par shift karna.
2. **Scalability**: Traffic badha toh horizontally 10 aur servers bina client ko pata chale add kar dena.
3. **Traffic Distribution**: Sabhi servers par equal/optimal load distribute karna.
4. **Security & Offloading**: SSL/TLS Termination, DDoS mitigation, Rate limiting offload karna.

---

## 2. Computer Networks Perspective: Layer 4 vs Layer 7 Load Balancer

TI (Texas Instruments) ya Core Systems interview me yeh **#1 Favorite Question** hota hai.

```
+-----------------------------------------------------------------------------+
| Layer 7 (Application LB) - Reads HTTP Path, Headers, Cookies, JSON body    |
| e.g., NGINX, HAProxy, Envoy, AWS ALB                                       |
+-----------------------------------------------------------------------------+
| Layer 4 (Transport LB)   - Reads only IP + TCP/UDP Port (No payload read)   |
| e.g., Linux IPVS/LVS, AWS NLB, HAProxy (TCP mode)                          |
+-----------------------------------------------------------------------------+
```

### In-Depth Comparison Table:

| Feature | Layer 4 Load Balancer (Transport) | Layer 7 Load Balancer (Application) |
| :--- | :--- | :--- |
| **OSI Layer** | Layer 4 (TCP / UDP) | Layer 7 (HTTP, HTTPS, gRPC, WebSocket) |
| **Data Inspected** | Source IP, Dest IP, Port, TCP Flags | URL Path (`/api/v1/users`), Headers, Cookies, Body |
| **TCP Termination** | 1 TCP connection direct ya NAT (No payload parsing) | 2 TCP connections (Client ↔ LB, and LB ↔ Backend) |
| **Performance** | Extremely Fast (Millions of packets/sec, low CPU) | Slower than L4 (Has to decrypt TLS, parse HTTP stream) |
| **Smart Routing** | Cannot route `/video` to Server A and `/pay` to B | Can route by URL path, method, auth token header |
| **Memory Footprint**| Very low (packet forwarding buffers) | High (buffer full HTTP headers & payload) |
| **DSR Support** | Supports **Direct Server Return** (DSR) | Cannot support DSR |

### 🔥 TI Killer Concept: Direct Server Return (DSR)
- Normal LB me Request LB se aati hai aur Response bhi LB ke through jaata hai. **Problem**: Response data (video/images) hamesha Request se 100x bada hota hai! LB bottleneck ban jayega.
- **DSR in L4**: Client request bhejta hai LB ko. LB sirf destination MAC address change karke packet Backend ko forward karta hai (IP same rehti hai). Backend sidha Client ko response bhej deta hai (LB bypass karke)! LB ka egress network load 95% kam ho jata hai!

---

## 3. Load Balancing Algorithms (Math & Logic)

### A. Round Robin & Weighted Round Robin
- **Round Robin**: Har request sequentially next server ko: `next = (current + 1) % N`.
  - *Concurrency Gotcha*: Multi-threaded environment me `current++` race condition create karta hai. `sync/atomic.AddUint64(&cursor, 1)` use karo.
- **Weighted Round Robin**: Agar Server A (32 Core) hai aur Server B (4 Core) hai:
  - Weight ratio A:3, B:1. Sequence: `A, A, A, B, A, A, A, B...`

### B. Least Connections
- Jis server ke active open connections sabse kam hain, use assign karo.
- Best for long-lived connections (WebSockets, Database connections, File uploads).

### C. Consistent Hashing (Must-Know for Distributed Systems)

#### Problem with Modulo Hashing (`hash(key) % N`):
Agar tumhare paas 4 servers hain ($N=4$) aur tum user ID ka hash karke server choose karte ho:
- `hash("user123") % 4 = 2` (Server 2)
- Agar 1 server crash ho gaya ($N=3$), toh lagbhag **100% keys ka hash badal jayega!** Sabhi cached sessions invalidate ho jayenge (Cache Stampede).

#### Consistent Hashing Solution:
1. Ek virtual ring hoti hai ($0$ se $2^{32}-1$).
2. Servers ko hash karke ring par place karte hain.
3. Request key ko hash karke ring par rakhte hain aur **clockwise** chalte hain jab tak pehla server na mil jaye.

```
               Hash Ring (0 to 2^32 - 1)
                      [Server A] (hash: 1000)
                     /          \
                    /            \
      (hash: 9000)                (hash: 3000)
    [Server C]                     [Request 1: hash 2500] ──► hits Server B
          \                        /
           \                      /
             [Server B] (hash: 4000)
```

- **Benefit**: Jab 1 server add ya delete hota hai, toh sirf $1/N$ fraction keys remap hoti hain, baaki sab unchanged rehti hain!
- **Virtual Nodes**: Non-uniform distribution ko prevent karne ke liye har physical server ke 100-200 virtual nodes ring par scatter karte hain (e.g. `ServerA-vnode1`, `ServerA-vnode2`).

---

## 4. Health Checking: Active vs Passive (Project Code Alignment)

### Active Health Check (Proactive Polling)
- Ek background goroutine/thread periodic interval par (e.g., every 5s) `/healthz` endpoint par GET request bhejti hai.
- Agar 3 consecutive failures hue $\rightarrow$ Server mark **DOWN**.
- *Code*: Hamare project me `health.NewHealthChecker` yahi karta hai.

### Passive Health Check (Inline Failure Detection)
- Real client traffic routing ke dauran agar reverse proxy ko `502 Bad Gateway` ya `Connection Refused` mila:
- Immediately server ko offline mark kar do bina next health check cycle ka wait kiye.
- *Code*: Hamare project me `proxy.NewProxy` ke error handler closure me `b.SetAlive(false)` call hota hai.

---

## 5. Reverse Proxy vs Forward Proxy vs API Gateway

```
[Client 1] ──┐
[Client 2] ──┼──► [FORWARD PROXY] ──► [ Internet / External Web ]
[Client 3] ──┘    (Hides Client IP, Bypasses Firewalls, Caches)

[ Internet Clients ] ──► [ REVERSE PROXY ] ──┬──► [ Backend Server A ]
                         (Hides Server IP,   ├──► [ Backend Server B ]
                          SSL, Load Balancing)└──► [ Backend Server C ]
```

- **Forward Proxy**: Client ke side baithta hai (e.g. Corporate proxy, VPN). Target server ko client ka actual IP nahi pata chalta.
- **Reverse Proxy**: Servers ke aage baithta hai. Client ko backend servers ki internal IPs aur topology nahi pata chalti.
- **API Gateway**: Reverse Proxy + Smart Business logic (JWT Authentication, Rate Limiting, Request Transformation, Canary Deployments, Metrics/Tracing).
