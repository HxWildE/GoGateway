# 04. ReverseProxy Internals & Closure Hooks

```
========================================================================================
                      REVERSE PROXY & PASSIVE FAILURE HOOK
========================================================================================

   [ Client HTTP Request ]
             │
             ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ proxy.GatewayHandler.ServeHTTP(w, r)                        │
   │                                                             │
   │ 1. backend := pool.GetNextBackend()                         │
   │ 2. backend.ReverseProxy.ServeHTTP(w, r)                     │
   └──────────────────────────────┬──────────────────────────────┘
                                  │
                                  ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ httputil.ReverseProxy (Standard Library Engine)             │
   │                                                             │
   │   ┌─────────────────────────────────────────────────────┐   │
   │   │ Director(req *http.Request)                         │   │
   │   │ - Rewrites req.URL.Scheme = "http"                  │   │
   │   │ - Rewrites req.URL.Host   = "127.0.0.1:8081"        │   │
   │   │ - Injects "X-Forwarded-For" header                  │   │
   │   └──────────────────────────┬──────────────────────────┘   │
   │                              │                              │
   │                              ▼                              │
   │   ┌─────────────────────────────────────────────────────┐   │
   │   │ http.DefaultTransport.RoundTrip(req)                │   │
   │   │ (Attempts real TCP socket connect to backend)       │   │
   │   └──────────────┬───────────────────────┬──────────────┘   │
   │                  │                       │                  │
   │            [ SUCCESS ]               [ FAILURE ]            │
   │                  │             (Connection Refused / 502)   │
   │                  ▼                       ▼                  │
   │      ┌──────────────────────┐┌──────────────────────────┐   │
   │      │ Stream Response Back ││ ErrorHandler(w, r, err)  │   │
   │      │ to Client Browser    ││ Closure captures `*b`    │   │
   │      └──────────────────────┘└───────────┬──────────────┘   │
   │                                          │                  │
   └──────────────────────────────────────────┼──────────────────┘
                                              │
                      ┌───────────────────────┘
                      ▼
   ┌─────────────────────────────────────────────────────────────┐
   │ CLOSURE CALLBACK:                                           │
   │ b.SetAlive(false)                                           │
   │ -> Instantly marks node dead without waiting for 5s Ticker! │
   │ -> Returns HTTP 502 Bad Gateway to client                   │
   └─────────────────────────────────────────────────────────────┘
```

---

## 🔬 Deep Dive: How Go Closure Captures the Pointer

```
main.go Scope:
┌─────────────────────────────────────────────────────────────────────────────┐
│ var b *loadbalancer.Backend                                                 │
│                                                                             │
│ proxyHandler := proxy.NewProxy(backendURL, timeout, func(err error) {       │
│     if b != nil {                                                           │
│         b.SetAlive(false)  <-- Closes over pointer variable `b` in heap      │
│     }                                                                       │
│ })                                                                          │
│                                                                             │
│ b = &loadbalancer.Backend{                                                  │
│     URL:          backendURL,                                               │
│     Alive:        true,                                                     │
│     ReverseProxy: proxyHandler,                                             │
│ }                                                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

```
💡 Why this is brilliant:
No circular dependency between package `loadbalancer` and package `proxy`!
Communication happens purely via Go first-class function closures.
```
