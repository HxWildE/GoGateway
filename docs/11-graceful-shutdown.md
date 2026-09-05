# 11. Graceful Shutdown Design

## 1. CONCEPT
**Graceful Shutdown** is the process of stopping a network server cleanly without abruptly terminating active client connections, leaking goroutines, or corrupting state.

---

## 2. WHY IT EXISTS
In cloud environments and container deployments (like Kubernetes or auto-scaling groups), nodes are frequently started, stopped, or replaced.
* If a server is terminated abruptly (SIGKILL or hard crash), any client currently uploading a file or executing a database write will experience connection resets and lost transactions.
* A professional backend system must drain active connections (finish processing in-flight work) while immediately refusing new incoming traffic.

---

## 3. HOW IT WORKS
The graceful shutdown sequence maps as follows:

```
[ OS Signal: SIGINT/SIGTERM ]
            │
            ▼
[ GatewayServer.Start() catches signal ]
            │
            ├─► 1. Stop HealthChecker Loop (Stop active pings)
            │
            ├─► 2. Call httpServer.Shutdown(ctx)
            │      │
            │      ├─► Immediately closes all network Listener ports
            │      │   (Incoming new clients receive connection refused)
            │      │
            │      └─► Waits for current active request goroutines to exit
            │
            ▼
[ Process exits cleanly ]
```

1. **Catch Signal**: The application intercepts OS termination signals (`os.Interrupt`, `syscall.SIGTERM`).
2. **Stop Background Workers**: Stop background loops (like health check tickers) so they don't spawn new tasks.
3. **Close Listener Sockets**: Stop accepting new TCP connections.
4. **Drain In-Flight Requests**: Allow active requests to finish processing.
5. **Enforce Timeout Context**: If some requests take too long, force-close connections after a safety timeout (e.g. 10s) to prevent the shutdown process from hanging indefinitely.

---

## 4. INTERNALS: GO'S `Shutdown()` MECHANISM
Go's `http.Server.Shutdown(ctx)` works under the hood by:
1. Closing all active listener sockets (`net.Listener`).
2. Closing all idle keep-alive connections immediately.
3. Waiting indefinitely for active connections to become idle or close.
4. If the passed `context.Context` is cancelled or reaches its deadline before all connections close, `Shutdown` returns the context's error, prompting the application to call `http.Server.Close()` to force-terminate all remaining sockets.

---

## 5. PROJECT USAGE
We orchestrate this in `server/server.go` inside the `Start()` method. We create a signal channel, notify it of `os.Interrupt` and `syscall.SIGTERM`, and block on `select` until a signal is received. We then pass a 10-second timeout context to `gs.httpServer.Shutdown(ctx)`.

---

## 6. CODE WALKTHROUGH
```go
package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func GracefulShutdownExample(srv *http.Server) {
	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Block until signal is received
	<-sigChan
	log.Println("Shutdown signal received, shutting down...")

	// Enforce 10-second maximum shutdown time
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Drain in-flight connections
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Forced close due to shutdown error: %v", err)
		srv.Close()
	}
}
```

---

## 7. RUNTIME FLOW
```
Client                     Proxy Listener Socket                  Request Handler
  │                                  │                                  │
  │─── HTTP GET /slow-report ───────►│                                  │
  │                                  │─── Forward Request ─────────────►│ (Processing...)
  │                                  │                                  │
  │                                  ◄─── OS SIGTERM received ──────────┤
  │                                  │ (Listener closed immediately;    │
  │                                  │  new requests fail to connect)   │
  │                                  │                                  │
  │                                  │                                  │ (Finished)
  │◄── HTTP 200 OK ──────────────────┼◄── Response Written ─────────────┤
  │                                  │                                  │
  └─ Connection closed gracefully ───┴──────────────────────────────────┘
```

---

## 8. FAILURE CASES
* **Hanging Connections (Slow/Malicious Client)**: A client starts downloading a large file extremely slowly. If the server waits forever, it will block deployments.
  * *Code Mitigation*: The context deadline (`context.WithTimeout`) guarantees that the proxy will force-shutdown and exit after the designated timeout (10 seconds), preventing hangs.
* **Shutting Down Health Checker After Server**: If the HTTP server is shut down before the background health checker, the checker will keep polling downstreams, generating errors, leaking goroutines, or writing to closed structures.
  * *Code Mitigation*: We call `gs.checker.Stop()` *before* invoking `httpServer.Shutdown(ctx)`.

---

## 9. TRADEOFFS
### Draining connections vs. Fast Restart
* **Graceful Draining (Default)**:
  * *Pros*: Protects customer requests; prevents incomplete writes/transactions; ensures clean API client experiences.
  * *Cons*: Retards container recycling; deployments take longer.
* **Immediate Kill (`http.Server.Close()`)**:
  * *Pros*: Instant process exit; fast restarts.
  * *Cons*: Drops active connections, causing client errors and half-written data.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: Why is graceful shutdown important in a containerized environment (like Kubernetes)?
   * **A**: Kubernetes constantly reschedules containers for rolling updates, resource balancing, and scaling. Without graceful shutdown, client requests hitting a terminating container are cut off instantly, leading to spikes in HTTP 5xx errors.
2. **Q**: Explain how `http.Server.Shutdown()` drains connections.
   * **A**: It closes all open listeners so new connections cannot be established. It then closes all idle keep-alive connections. It waits for active requests to finish and close their connections. When the active connection count hits zero, it returns.
3. **Q**: What is the purpose of passing a Context with a timeout to `Shutdown()`?
   * **A**: To enforce a deadline. If a client connection is stuck or downloading extremely slowly, the server would hang forever waiting for it to finish. The timeout context ensures the server force-terminates remaining connections and exits after a defined safety duration.
