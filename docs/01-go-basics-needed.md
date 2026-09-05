# 01. Go Basics Needed for Systems Engineering

## 1. CONCEPT
To build low-level network servers in Go, you must understand how Go handles memory, structural data, packages, and code visibility. Specifically, you need to master **Pointers**, **Structs**, **Visibility (Capitalization)**, **Interfaces**, and **Module/Package Imports**.

---

## 2. WHY IT EXISTS
Systems engineering requires direct, efficient access to data, modular code separation, and explicit memory semantics.
* **Pointers** exist to let you reference memory directly and pass variables by reference, avoiding expensive copies of large data structures.
* **Structs** group related fields together to model complex entities (like backend servers or connection pools).
* **Visibility rules** enforce encapsulation at the package boundary.
* **Interfaces** allow decoupled, polymorphic behavior (like abstracting the load-balancing strategy).

---

## 3. HOW IT WORKS
* **Pointers**: A pointer stores the memory address of a value. The `*` operator declares a pointer type or dereferences a pointer to get the underlying value. The `&` operator generates a pointer to an existing variable.
* **Structs**: Defined using `type Name struct { ... }`. They are instantiated using struct literals. Methods are attached to structs using value or pointer receivers.
* **Visibility**: Capitalized names (functions, structs, fields) are exported and accessible outside their package. Lowercase names are unexported (private to the package).
* **Interfaces**: A type implements an interface implicitly by defining all the interface's methods. No explicit `implements` keyword is used.
* **Packages**: Reusable blocks of code that belong together. Packages are imported using paths relative to the module name defined in `go.mod`.

---

## 4. INTERNALS
Under the hood:
* **Pointers & Escape Analysis**: The Go compiler decides whether to allocate a variable on the **Stack** (fast, cleaned up on function return) or the **Heap** (slower, managed by Garbage Collector). If a variable's address is shared outside the function call stack (e.g., returned as a pointer), the compiler "escapes" it to the heap.
* **Interfaces**: An interface value is represented internally as a two-pointer structure: `(tab, data)`. `tab` points to the Interface Table (Itable), which stores the concrete type's runtime metadata and function pointers. `data` points to the actual concrete value.

---

## 5. PROJECT USAGE
In our Reverse Proxy + Load Balancer, we use:
* **Pointers**: To pass the `Backend` and `BackendPool` structs around without copying, ensuring modifications (e.g., updating health status) reflect globally.
* **Struct receivers**: For encapsulating behavior (e.g., `func (b *Backend) SetAlive(alive bool)`).
* **Unexported fields**: To prevent other packages from modifying internal states directly (e.g., `Backend.Alive` is unexported and protected by methods).

---

## 6. CODE WALKTHROUGH
Here is how we use pointers, structs, and methods in our load balancer:

```go
package loadbalancer

import (
	"net/url"
	"sync"
)

// Struct grouping related data
type Backend struct {
	URL   *url.URL     // Pointer to url.URL struct (avoiding copy)
	Alive bool         // Health state of backend
	mu    sync.RWMutex // Protects health state in concurrent environments
}

// Method with a Pointer Receiver (*Backend)
// Using a pointer receiver allows us to modify the fields of the struct.
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	b.Alive = alive // Modifies the actual struct, not a copy
	b.mu.Unlock()
}
```

---

## 7. RUNTIME FLOW
```
[ Caller ] 
    │ 
    │ passes pointer &b (e.g. 0xc0000840c0)
    ▼
[ SetAlive(alive bool) ]
    │
    ├─► Locks b.mu (0xc0000840c8)
    ├─► Modifies memory at 0xc0000840c0 (sets Alive = true/false)
    └─► Unlocks b.mu
```

---

## 8. FAILURE CASES
* **Nil Pointer Dereference**: If a pointer is declared but not initialized, its value is `nil`. Trying to read or write fields of a nil pointer crashes the program with a runtime panic: `panic: runtime error: invalid memory address or nil pointer dereference`.
  * *Code Mitigation*: Always initialize structs using `&Backend{}` or constructors like `NewBackendPool()`, and verify pointers before dereferencing if they can be nil.

---

## 9. TRADEOFFS
### Pointer Receiver vs. Value Receiver
* **Pointer Receiver (`*Backend`)**:
  * *Pros*: Can modify the original struct's data; avoids copying large structs (performance gain under high load).
  * *Cons*: Requires synchronization (like mutexes) if accessed concurrently, because multiple goroutines can write to the same memory.
* **Value Receiver (`Backend`)**:
  * *Pros*: Completely thread-safe because the method works on a copy of the struct; no mutexes needed for read-only copies.
  * *Cons*: Cannot update the state of the original caller; high copying cost if the struct holds many fields.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: What is the difference between passing a variable by value and passing it by pointer in Go?
   * **A**: Passing by value copies the entire data, meaning changes inside the function do not affect the original variable. Passing a pointer copies the memory address of the variable. Changes made through the pointer modify the original variable.
2. **Q**: How does Go's compiler decide whether to put a variable on the stack or the heap?
   * **A**: Go uses **Escape Analysis** at compile time. If the compiler can prove that a variable does not outlive the lifetime of the stack frame in which it was created, it allocates it on the stack. Otherwise, it escapes to the heap.
3. **Q**: What are unexported fields in Go, and how are they enforced?
   * **A**: Fields or identifiers starting with a lowercase letter are unexported. This means they are only visible and accessible inside the package where they are defined. The compiler strictly enforces this at compile time.
