# 03. TCP and Networking Fundamentals

## 1. CONCEPT
A Reverse Proxy sits at the intersection of multiple network connections. To explain it in an interview, you must understand the **Client/Server Model**, **IP Addresses/Ports**, **Sockets**, **TCP Connection Management (3-way handshake)**, and the separation of client-to-proxy and proxy-to-backend communication legs.

---

## 2. WHY IT EXISTS
Computers communicate across networks using layered protocols.
* **IP Addresses** identify hosts on a network.
* **Ports** identify specific applications running on a host.
* **Sockets** are the software abstractions allowing applications to read/write network data.
* **TCP (Transmission Control Protocol)** provides a reliable, ordered, and error-checked byte stream. HTTP relies on TCP to ensure that requests and responses are delivered intact.

---

## 3. HOW IT WORKS
### TCP 3-Way Handshake
Before sending HTTP data, the client and server must establish a TCP connection:

```
Client                               Server
  │                                     │
  │ ─── SYN (Sequence = X) ────────────►│  (1. "I want to connect")
  │                                     │
  │◄─── SYN-ACK (Seq = Y, Ack = X+1) ───┤  (2. "I accept, let's sync")
  │                                     │
  │ ─── ACK (Sequence = X+1, Ack = Y+1)►│  (3. "Acknowledged, connected")
  │                                     │
  ▼                                     ▼
[ Established ]                  [ Established ]
```

### The Sockets Abstraction
A socket is defined by a 5-tuple:
`(Protocol, Source IP, Source Port, Destination IP, Destination Port)`
When a client sends a request to the proxy, a TCP socket is opened. The proxy then establishes a *second*, independent TCP socket to forward the request to the backend.

---

## 4. INTERNALS
### HTTP Keep-Alive / Persistent Connections
Establishing a TCP connection via the 3-way handshake adds latency (1.5 round-trip times - RTT). To optimize performance:
* **HTTP Keep-Alive** allows reuse of the same underlying TCP connection for multiple HTTP requests.
* The connection is kept open after a response.
* In Go, the `Transport` layer automatically maintains a pool of idle TCP connections (`MaxIdleConns`) to downstream backends to eliminate handshake overhead for future requests.

---

## 5. PROJECT USAGE
Our gateway has two independent network legs:

```
    CLIENT LEG (Leg 1)                    BACKEND LEG (Leg 2)
[ Client ] ───────────────► [ Gateway Proxy ] ───────────────► [ Backend A ]
  Host: client.local          Host: gateway:8080               Host: backend:8081
  Socket:                     Socket:                          Socket:
  ClientIP:Port ◄──► ProxyIP:8080      ProxyIP:Port ◄──► BackendIP:8081
```

The gateway reads data from Leg 1, routes it, opens/reuses a socket on Leg 2, forwards the request, reads the response from Leg 2, and writes it back to Leg 1.

---

## 6. CODE WALKTHROUGH
The creation of the second network leg is managed inside `proxy/proxy.go` using a custom `http.Transport`:

```go
package proxy

import (
	"net"
	"net/http"
	"net/url"
	"time"
)

func ConfigureTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,         // Maximum time to wait for TCP handshake (SYN -> SYN-ACK -> ACK)
			KeepAlive: 30 * time.Second, // Interval for TCP keep-alive probes
		}).DialContext,
		MaxIdleConns:    100,              // Keep up to 100 idle TCP connections pooled
		IdleConnTimeout: 90 * time.Second, // Close backend connection if idle for 90s
	}
}
```

---

## 7. RUNTIME FLOW
1. **Client** initiates a TCP 3-way handshake with the **Gateway** on port `:8080`.
2. **Gateway** accepts the connection, reading the client's HTTP request stream.
3. **Gateway** selects a backend (e.g. `:8081`) and checks if there is an idle TCP connection in its pool.
4. If no idle connection exists, **Gateway** performs a *new* TCP 3-way handshake with the **Backend** on port `:8081`.
5. **Gateway** forwards the HTTP request headers and body.
6. **Backend** processes the request and sends the response.
7. **Gateway** relays the response to the Client and returns the backend connection to its connection pool.

---

## 8. FAILURE CASES
* **TCP Connection Timeout**: If the backend server is dead or firewalled, the TCP SYN packet goes unacknowledged. The proxy will hang trying to establish a connection.
  * *Code Mitigation*: We set `DialContext.Timeout = timeout` inside `Transport` so the connection attempt fails quickly, allowing the proxy to mark the backend dead and return a `502 Bad Gateway` to the client.
* **TCP Half-Open Connections**: If a client abruptly disconnects (e.g. losing cell service), the socket remains "open" on the proxy server, consuming resources.
  * *Mitigation*: TCP Keep-Alive probes (`net.Dialer.KeepAlive`) run periodically to detect broken links and close dead sockets.

---

## 9. TRADEOFFS
### Persistent Connections (Keep-Alive) vs. Connection-Close
* **Persistent Connections (Default)**:
  * *Pros*: Eliminates 3-way handshake latency (RTT) for subsequent requests; reduces packet overhead.
  * *Cons*: Requires server memory to maintain socket states; susceptible to resource exhaustion if too many idle connections are held.
* **Connection-Close**:
  * *Pros*: Cleans up resources immediately after a request; no idle connection tracking.
  * *Cons*: Adds TCP connection overhead to every request; destroys throughput performance.

---

## 10. INTERVIEW QUESTIONS
1. **Q**: Describe the TCP 3-way handshake.
   * **A**: The client sends a `SYN` (Synchronize) packet to the server to initiate a connection. The server responds with a `SYN-ACK` packet acknowledging the sequence number and sending its own. The client replies with an `ACK` (Acknowledge) packet. The connection is now established.
2. **Q**: Why does a reverse proxy maintain a separate connection pool for backends?
   * **A**: A reverse proxy acts as an intermediary, isolating clients from backend infrastructure. By maintaining a separate connection pool for backends, it can multiplex requests from thousands of slow clients over a few fast, pre-established persistent TCP connections to backends, preventing backend socket exhaustion.
3. **Q**: What is the difference between a TCP socket and a port?
   * **A**: A port is a logical address on a host (a 16-bit number) representing a communication channel. A socket is the actual runtime endpoint formed by binding an IP address and a port. An active TCP connection is defined by a socket pair (client socket and server socket).
4. **Q**: What is TCP head-of-line blocking?
   * **A**: In TCP, if a packet is lost in transit, all subsequent packets must wait in the receiver's buffer until the lost packet is retransmitted and acknowledged. This stalls transmission across the socket, which is known as head-of-line blocking.
