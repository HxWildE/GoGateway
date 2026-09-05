# 04. Structs, HTTP Handlers & Closures Explained 🧩

Go me Web Servers aur Load Balancers create karte waqt 3 sabse important programming paradigms use hote hain:
1. **Interfaces & Structs** (`http.Handler`)
2. **Reverse Proxying Layer** (`httputil.ReverseProxy`)
3. **Closures (Anonymous Callbacks)** passive failure handling ke liye.

Is chapter me hum dekhenge ki in teeno concepts ne **GoGateway** ko kitna clean aur modular banaya hai.

---

## 1. What is an HTTP Handler in Go? (`http.Handler` Interface)

Go ki standard library `net/http` me **`http.Handler`** ek core interface hai:

```go
// Go Standard Library Definition
type Handler interface {
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```

Iska matlab: **Koi bhi struct (ya object) jo `ServeHTTP` method implement kar le, wo Go ka valid Web Server Router/Handler ban sakta hai!**

### C++ Comparison:
C++ me aap pure virtual class banate:
```cpp
class HttpHandler {
public:
    virtual void serveHTTP(HttpResponse& w, HttpRequest& r) = 0;
};
```

### Hamare Gateway Ka Custom Handler ([proxy/proxy.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/proxy/proxy.go#L65-L97))

Humne `GatewayHandler` struct banayi jo `http.Handler` interface ko fulfill karti hai:

```go
type GatewayHandler struct {
	Pool *loadbalancer.BackendPool
}

func (gh *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Next healthy backend select karo pool se
	targetBackend := gh.Pool.NextBackend()
	if targetBackend == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "Service Unavailable: No healthy backends\n")
		return
	}

	// 2. Client HTTP Request ko target backend ke ReverseProxy par forward kardo!
	targetBackend.ReverseProxy.ServeHTTP(w, r)
}
```

```
┌────────────────────────────────────────────────────────────────────────┐
│                      HTTP HANDLER DELEGATION FLOW                      │
│                                                                        │
│  Client Request GET /api/data  ──►  net/http Server Listening on :8080 │
│                                                │                       │
│                                                ▼                       │
│                             GatewayHandler.ServeHTTP(w, r)             │
│                                                │                       │
│                                                ▼                       │
│                             Select Next Backend via Atomic             │
│                                                │                       │
│                                                ▼                       │
│                           targetBackend.ReverseProxy.ServeHTTP()       │
│                                                │                       │
│                                                ▼                       │
│                           TCP Forward to Backend Server (:8081)        │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Reverse Proxy Layer & Header Modulation

[proxy/proxy.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/proxy/proxy.go#L15-L62) me `NewProxy` function ek smart `httputil.ReverseProxy` instance create karta hai:

```go
func NewProxy(target *url.URL, timeout time.Duration, onFailure func(error)) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Custom HTTP Transport with Timeouts & Connection Pooling
	proxy.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,         // TCP connection timeout (e.g. 2s)
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:    100,               // Connection Pool Size
		IdleConnTimeout: 90 * time.Second,
	}

	// Request Director: Proxying se pehle client ke original headers attach karna
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if req.Header.Get("X-Forwarded-Host") == "" {
			req.Header.Set("X-Forwarded-Host", req.Host) // Preserve Original Host
		}
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	return proxy
}
```

---

## 3. The Closure Trick for Passive Failure Detection 🪄

Passive failure detection ka matlab hai: **Jaise hi Reverse Proxy me koi backend request timeout ya connection crash hota hai, hum uss backend ko IMMEDIATELY offline mark kar dete hain, bina 2-second health check ticker ka wait kiye!**

Ye humne ek **Closure (Callback Function)** ki madad se achieve kiya hai!

### [main.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/main.go#L58-L74) me Closure Construction:

```go
// 1. Declare pointer variable first
var b *loadbalancer.Backend

// 2. Define Proxy with Anonymous Closure Function
proxyHandler := proxy.NewProxy(backendURL, cfg.ProxyTimeout, func(err error) {
	if b != nil {
		// Closure outer scope ke 'b' variable ko memory me capture kar leta hai!
		b.SetAlive(false)
	}
})

// 3. Instantiate Backend struct with pointer reference
b = &loadbalancer.Backend{
	URL:          backendURL,
	Alive:        true,
	ReverseProxy: proxyHandler,
}
```

### [proxy/proxy.go](file:///c:/Users/harsh/OneDrive/Documents/Desktop/MYwebDEvprojects/LEtsGoLAng/proxy/proxy.go#L53-L59) me Triggering:

```go
proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("[Proxy] Error routing request to %s: %v. Marking backend offline.", target.String(), err)
	
	onFailure(err) // Calls the closure passed from main.go!

	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, "Bad Gateway: Failed to connect to downstream backend server.\n")
}
```

```
┌────────────────────────────────────────────────────────────────────────┐
│                   PASSIVE FAILURE CLOSURE EXECUTION                    │
│                                                                        │
│  1. Client Request ──► ReverseProxy ──► Backend Server (Crashed/500)   │
│                                               │                        │
│  2. Network Error! ───────────────────────────┘                        │
│         │                                                              │
│         ▼                                                              │
│  3. proxy.ErrorHandler fires                                           │
│         │                                                              │
│         ▼                                                              │
│  4. Triggers closure: onFailure(err)                                   │
│         │                                                              │
│         ▼                                                              │
│  5. Closure executes outer scope code: b.SetAlive(false)               │
│         │                                                              │
│         ▼                                                              │
│  6. Backend marked OFFLINE IMMEDIATELY! Next requests bypass it.      │
└────────────────────────────────────────────────────────────────────────┘
```

### 🧠 Closure Concept Explained:
Closure ek aisa function hota hai jo apne bahar (outer lexical scope) ke variables ko "bind" kar ke apne andar save kar leta hai.

C++ me ise Lambda with Capture Clause kehte hain:
```cpp
Backend* b = nullptr;
auto onFailure = [&b](const std::exception& err) {
    if (b != nullptr) b->setAlive(false);
};
```

Go me `func(err error)` intuitively outer variable `b` ki reference hold kar leta hai. Isse `proxy.go` package ko `loadbalancer.Backend` ke internal structure ka gyaan rakhne ki zaroorat nahi rehti — High Decoupling & Clean Architecture!
