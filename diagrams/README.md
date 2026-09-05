# 🎨 Visual Diagram Master Hub (Daily 5-Min Memory System)

> **Kyu banaya ye folder?**
> System Design aur Low-Level Architecture ko rattne se achha hai unke **visual blueprints** ko roz 3-5 din tak subah 5 minute dekhna. Dimag naturally visual blocks aur connections ko memorize kar leta hai!

---

## 🗺️ Master Visual Directory

| File | Topic & Diagram Focus | Format Available |
| :--- | :--- | :--- |
| **[01-system-architecture.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/01-system-architecture.md)** | **End-to-End System HLD & LLD** (Client $\rightarrow$ Gateway $\rightarrow$ ReverseProxy $\rightarrow$ Backends) | Mermaid + ASCII + Component Breakdown |
| **[02-concurrency-and-goroutines.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/02-concurrency-and-goroutines.md)** | **Goroutine Lifecycles & GMP Scheduler** (Main, Backend spawns, Ticker, Channels, Workers) | Mermaid Sequence + Concurrency Flow |
| **[03-load-balancer-and-memory.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/03-load-balancer-and-memory.md)** | **Atomic Operations, Mutex Locks & Consistent Hash Ring** (Memory layout, CPU Cache line) | Memory Map + Visual Hash Ring |
| **[04-proxy-and-error-closures.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/04-proxy-and-error-closures.md)** | **ReverseProxy Internals & Closure Hooks** (`httputil.ReverseProxy`, ErrorHandler callback) | Data Flow + Closure Scope Pointer |
| **[05-rate-limiting-and-algorithms.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/05-rate-limiting-and-algorithms.md)** | **5 Rate Limiting Algorithms & Distributed Redis Architecture** (Token Bucket, Leaky, Sliding Window) | Algorithm Step Graphs + Redis Lua |
| **[06-networking-and-socket-layer.md](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/06-networking-and-socket-layer.md)** | **OSI Layer 4 vs 7, Kernel Sockets & Epoll Event Loop** (TCP 3-Way Handshake, DSR, TIME_WAIT) | Network Packet Trace + Kernel Ring |
| **[architecture-overview.excalidraw](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/architecture-overview.excalidraw)** | **Interactive Excalidraw Canvas** (VS Code extension / [excalidraw.com](https://excalidraw.com)) | Native Excalidraw JSON |
| **[rate-limiter-and-loadbalancer.excalidraw](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/diagrams/rate-limiter-and-loadbalancer.excalidraw)** | **Interactive Excalidraw Canvas 2** (Algorithms + Sockets + Dual Health Checks) | Native Excalidraw JSON |

---

## ⚡ How to Use Excalidraw Files
1. **Option A (VS Code / Antigravity IDE)**: Install `Excalidraw` extension and simply click on `*.excalidraw` files. It renders the full interactive whiteboard natively!
2. **Option B (Web Browser)**: Open [excalidraw.com](https://excalidraw.com), click the folder icon $\rightarrow$ "Open", aur `.excalidraw` file choose karo ya contents paste kar do!

---

## 🚀 3-Day Memorization Technique
- **Day 1**: 01-system-architecture & 02-concurrency-and-goroutines dekh ke mentally request trace karo.
- **Day 2**: 03-load-balancer-and-memory & 04-proxy-and-error-closures me pointers aur mutex locking sequence ko visualize karo.
- **Day 3**: 05-rate-limiting & 06-networking me Token Bucket math aur OSI L4 vs L7 comparison table memorize karo.
