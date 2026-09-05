# OOP Principles & Design Patterns in Go (Project Context)

Go language ek traditional OOP (Object-Oriented Programming) language nahi hai (like Java ya C++). Isme `class`, `extends`, ya `implements` jaise keywords nahi hote. 

Lekin, Go OOP ke **Core Principles** ko ek alag aur behtar tareeqe se support karta hai. Is project me humne OOPs aur SOLID principles ka heavy use kiya hai bina traditional classes ke. 

Neeche har concept ki definition aur uski **exact implementation** project ke code ke saath samjhayi gayi hai.

---

## 1. Classes & Objects (Structs in Go)
Go me data aur state ko hold karne ke liye `class` nahi, balki `struct` use hota hai. Ek struct ke instance ko hum Object bol sakte hain.

**Pattern:** Defining blueprints for data.
**In Our Project:** Humne alag-alag entities ke liye structs banaye hain.

```go
// loadbalancer/roundrobin.go
// Yeh ek "Class" blueprint ki tarah hai
type Backend struct {
	URL          *url.URL
	Alive        bool
	ReverseProxy *httputil.ReverseProxy
	mu           sync.RWMutex
}

// main.go me object creation (Instantiation)
b := &loadbalancer.Backend{
    URL:          backendURL,
    Alive:        true,
    ReverseProxy: proxyHandler,
}
```

---

## 2. Encapsulation (Data Hiding)
Encapsulation ka matlab hai internal state ko bahari duniya se chupana taaki koi use galat tareeqe se modify na kar de. Go me `public/private` keywords nahi hote. 
* Agar variable ka naam **Capital letter** se shuru hota hai (e.g., `Alive`), toh wo **Public** (Exported) hai.
* Agar variable ka naam **Small letter** se shuru hota hai (e.g., `backends`), toh wo **Private** (Unexported) hai, aur doosre package se directly access nahi ho sakta.

**Pattern:** Hiding internal state and providing safe access via methods.
**In Our Project:** `BackendPool` me index aur list hide ki gayi hai.

```go
// loadbalancer/roundrobin.go
type BackendPool struct {
	backends []*Backend // Private: Bahar ka code list me directly cheezien add/remove nahi kar sakta
	current  uint64     // Private: Koi direct counter ko modify karke race condition nahi bana sakta
}

// Public Method to access private state safely
func (bp *BackendPool) GetBackends() []*Backend {
	return bp.backends
}
```

---

## 3. Methods (Behaviors / Member Functions)
Go me functions ko structs ke saath attach karne ke liye **Receiver Functions** use hote hain. Ye traditional OOP languages ke methods ki tarah behave karte hain.

**Pattern:** Binding behavior to data.
**In Our Project:** `NextBackend` method `BackendPool` object se juda hua hai.

```go
// loadbalancer/roundrobin.go
// (bp *BackendPool) "Receiver" kehlata hai.
// Yeh banata hai NextBackend() ko BackendPool class ka ek method.
func (bp *BackendPool) NextBackend() *Backend {
	next := atomic.AddUint64(&bp.current, 1)
	// ... logic
}

// Usage:
// pool.NextBackend() // Calling method on object
```

---

## 4. Polymorphism & Duck Typing (Interfaces)
Polymorphism ka matlab hai ek hi interface ke multiple forms hona. Go me interfaces **implicit** hote hain. Tumhe `implements` keyword likhne ki zaroorat nahi. Agar tumhara struct interface ke saare methods satisfy karta hai, toh wo automatically us interface type ka ban jata hai (Duck Typing).

**Pattern:** Designing for behavior, not concrete types.
**In Our Project:** `GatewayHandler` struct standard library ke `http.Handler` interface ko implement karta hai.

```go
// net/http standard library ka interface:
// type Handler interface {
//     ServeHTTP(ResponseWriter, *Request)
// }

// proxy/proxy.go
type GatewayHandler struct {
	Pool *loadbalancer.BackendPool
}

// Humne ServeHTTP method attach kiya. Ab GatewayHandler implicitly ek http.Handler ban gaya.
func (gh *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // routing logic...
}

// server/server.go
// Server kisi bhi object ko accept kar lega jiske paas ServeHTTP method ho! (Polymorphism)
httpServer := &http.Server{
    Addr:    addr,
    Handler: handler, // gatewayHandler yahan pass hota hai
}
```

---

## 5. Composition over Inheritance
Go me Inheritance (`extends`) nahi hota. Go strict principle follow karta hai: "Favor Composition over Inheritance". Iska matlab ek bada object chhote objects ko apne andar compose (embed) karke banta hai.

**Pattern:** A "Has-A" relationship instead of "Is-A" relationship.
**In Our Project:** `GatewayServer` inherit nahi karta kisi HTTP Server class ko, balki uske andar ek `http.Server` hota hai.

```go
// server/server.go
type GatewayServer struct {
    // GatewayServer "has a" http.Server, it is NOT an http.Server
	httpServer      *http.Server
    // GatewayServer "has a" HealthChecker
	checker         *health.HealthChecker
	shutdownTimeout time.Duration
}
```

---

## 6. Dependency Injection (DI)
Yeh SOLID principles ka 'D' (Dependency Inversion) hai. Hardcoding dependencies (ki ek object doosre ko khud create kare) ki jagah, dependencies bahar se pass ki jati hain constructor me. Isse system highly testable banta hai.

**Pattern:** Inject dependencies via constructors.
**In Our Project:** `GatewayServer` aur `GatewayHandler` dono Dependency Injection use karte hain.

```go
// server/server.go
// Bad Practice (Tight Coupling): 
// checker = health.NewHealthChecker() andar hi likh dena.

// Good Practice (Dependency Injection):
// Handler aur Checker bahar (main.go) se pass kiye ja rahe hain.
func NewGatewayServer(addr string, handler http.Handler, checker *health.HealthChecker, shutdownTimeout time.Duration) *GatewayServer {
	return &GatewayServer{
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
		checker:         checker,
		shutdownTimeout: shutdownTimeout,
	}
}
```

---

## 7. The Singleton Pattern (Concepts applied)
Strictly Singleton pattern ka matlab hai pure process me ek class ka sirf ek hi instance hona. Go me usually log global variables avoid karte hain aur struct pointers ko pass karna prefer karte hain.

**Pattern:** Ensuring only one instance of an object manages a state.
**In Our Project:** Humara `BackendPool` aur `HealthChecker` effectively singleton ki tarah behave karte hain kyunki `main.go` me inka sirf ek hi instance banaya gaya hai aur wahi same instance baaki sab me reference (pointer) ke roop me inject kiya gaya hai.

```go
// main.go
// Ek hi pool banaya
pool := loadbalancer.NewBackendPool()

// Wahi ek pool Proxy ko diya
gatewayHandler := proxy.NewGatewayHandler(pool)

// Wahi same pool HealthChecker ko diya
checker := health.NewHealthChecker(pool, cfg.HealthCheckInterval, cfg.HealthCheckTimeout)
```

---

## Interview Cheat Sheet Summary

Agar Interviewer pooche: *"Tell me about the object-oriented design patterns you used in your Go Gateway?"*

**Aapka Jawab:**
*"Go is not a traditional OOP language, but it provides powerful tools to enforce OOP principles. In my Load Balancer:*
1.  *I used **Structs and Receiver Functions** to model entities like `BackendPool` and `HealthChecker`, grouping state and behavior (Classes/Objects).*
2.  *I used **Encapsulation** by keeping slices and counters unexported (lowercase variables) in `BackendPool`, protecting them from external modification and preventing race conditions.*
3.  *I achieved **Polymorphism** using Go's implicit interfaces (Duck Typing). My `GatewayHandler` struct implements the `http.Handler` interface by providing a `ServeHTTP` method, allowing the standard `net/http` server to route traffic to it without tightly coupling.*
4.  *I favored **Composition over Inheritance** by embedding the standard `http.Server` inside my custom `GatewayServer` struct.*
5.  *Finally, I heavily used **Dependency Injection**. The orchestrator (`main.go`) creates the pool and passes it via pointers into the health checker and gateway handler, making the components decoupled and easily unit-testable."*
