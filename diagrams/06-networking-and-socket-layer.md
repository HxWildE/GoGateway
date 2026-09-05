# 06. Computer Networks, OSI Stack & Kernel Socket Internals

```
========================================================================================
                      LAYER 4 VS LAYER 7 SOCKET FLOW
========================================================================================

           LAYER 4 LOAD BALANCING (Transport Level - IP / Port)
┌────────┐               ┌─────────────────┐               ┌─────────┐
│ Client │ ────────────► │ L4 LB (IPVS/NLB)│ ────────────► │ Backend │
└────────┘               └─────────────────┘               └─────────┘
              Single TCP Connection (Packet rewriting / NAT / DSR)
              ❌ Does NOT decrypt TLS
              ❌ Does NOT read HTTP Path / Headers / Cookies


           LAYER 7 REVERSE PROXY (Application Level - HTTP / gRPC)
┌────────┐   TCP Conn 1  ┌─────────────────┐   TCP Conn 2  ┌─────────┐
│ Client │ ────────────► │ L7 LB (Gateway) │ ────────────► │ Backend │
└────────┘               └─────────────────┘               └─────────┘
              1. Terminates Client TCP & Decrypts TLS
              2. Inspects Request Headers & Path (/api/v1/orders)
              3. Initiates 2nd TCP Conn to internal Backend
              4. Buffers & streams response back
```

---

## 🔌 Kernel Sockets, epoll & Buffer Internals

```
                          LINUX KERNEL SPACE
┌─────────────────────────────────────────────────────────────────────────────┐
│ Socket FD 3 (Client Conn)        Socket FD 4 (Backend Conn)                 │
│ ┌─────────────────────────┐      ┌─────────────────────────┐                │
│ │ Receive Buffer (SO_RCV) │      │ Send Buffer (SO_SND)    │                │
│ │ [ Byte 1 | Byte 2 | ... ]│     │ [ Byte 1 | Byte 2 | ... ]│               │
│ └────────────┬────────────┘      └────────────▲────────────┘                │
│              │                                │                             │
│              │        Linux `epoll` Loop      │                             │
│              └───────────────► ◄──────────────┘                             │
│                                                                             │
└──────────────────────────────┬──────────────────────────────────────────────┘
                               │ Zero-Copy splice() or Go netpoller
                               ▼
                          USER SPACE
┌─────────────────────────────────────────────────────────────────────────────┐
│  Go Runtime netpoller (Parks Goroutine when socket buffer is empty/full)    │
│  Zero thread context switching overhead!                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## ⚡ Direct Server Return (DSR) in Layer 4

```
                          DIRECT SERVER RETURN (DSR)
                          
                          [ Client Browser ]
                             ▲          │
                             │          │ 1. Small Request (1 KB)
         3. High-Bandwidth   │          │    Dest IP: VIP (198.51.100.1)
            Direct Response  │          ▼
            (10 MB Video)    │   ┌──────────────┐
                             │   │ L4 Balancer  │ (Rewrites Dest MAC addr only)
                             │   └──────┬───────┘
                             │          │ 2. Forwarded Packet
                             │          ▼
                             └─── [ Backend Server ]
                                  (Configured with VIP on Loopback interface)
```
