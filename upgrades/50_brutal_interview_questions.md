# 50 Brutal Interview Questions & Answers

Yeh list Texas Instruments, Amazon, aur Uber jaisi companies ke Core Systems / Backend interviews me puche jane wale sawaalon ka collection hai. Tumhara code chalna kaafi nahi hai, tumhe usey "defend" karna aana chahiye.

---

## Part 1: Go Concurrency & Synchronization (The Hot Path)

**1. Q: Tumne round-robin selection ke liye `sync/atomic` use kiya mutext kyun nahi?**
* **A:** Load balancer ka selection logic "Hot Path" par hota hai (har ek request par run hota hai). Agar main `sync.Mutex` use karta, toh hazaaron concurrent requests aapas me lock ke liye ladti (lock contention), jisse system me context switching overhead badh jata. `atomic.AddUint64` hardware level par atomic instruction use karta hai jo lock-free hota hai aur contention avoid karta hai. CPU level par ye kaafi fast hai.

**2. Q: Atomic toh thik hai, but index calculation `idx = counter % len(backends)` me kya problem ho sakti hai?**
* **A:** Agar `len(backends)` dynamic ho (backends add/remove ho rahe ho) aur request ke beech me length change ho jaye, toh race condition ho sakti hai jisse panic (index out of bounds) aayega. Current design me backends list start me fixed hai. Agar dynamic karni ho, toh mujhe slice ko atomic pointer se replace karna padega ya RWMutex lagani padegi list read karte waqt.

**3. Q: Health check (Alive boolean) par `sync.RWMutex` kyun lagaya, normal `Mutex` kyun nahi?**
* **A:** `Alive` status ko read karna hazaaron requests (Gateway handler) ko karna hota hai ek hi waqt pe, jabki usey write/modify sirf Health Checker goroutine (har 5 sec me ek baar) ya failure callback karta hai. Normal Mutex read karne par bhi baki readers ko block kar deta. `RWMutex` allows multiple goroutines to read `Alive` simultaneously without blocking each other. Block sirf tab hota hai jab write ho raha ho.

**4. Q: Kya `RWMutex` me "Writer Starvation" ho sakti hai tumhare design me?**
* **A:** Yes. Agar continuous readers (incoming heavy traffic) RLock lagate rahein, toh writer (health checker) ko lock milne me delay ho sakta hai Go ke purane versions me. But Go ke modern scheduler me fairness build ki gayi hai, writer arrive hone par naye readers block ho jate hain taaki writer starvation na ho.

**5. Q: Tumne passive failure callback ek closure banaya. Closure function closure environment me bahar ke variable (`*Backend`) ko kaise retain karta hai? Memory leak toh nahi hai?**
* **A:** Go me closure jab create hota hai, toh wo heap par escape kar jata hai agar uski life outer scope se zyada ho. `b *Backend` closure me capture hua hai. Kyunki ye pointer proxy error handler ke saath bind hai aur jab tak wo backend proxy memory me hai, tab tak ye heap par zinda rahega. Ye memory leak nahi hai, ye deliberate state capture hai.

**6. Q: Go's scheduler goroutines ko OS threads pe kaise map karta hai?**
* **A:** Go M:N scheduling model (G-M-P model) use karta hai. M = OS Threads, G = Goroutines, P = Logical Processors. Agar ek goroutine network I/O par block hoti hai, toh P us thread (M) ko I/O poller ke hawale karke naya M le leta hai taaki baaki Gs execute hoti rahein. Isiliye hum millions of concurrent connections handle kar sakte hain bina thousands of OS threads banaye.

---

## Part 2: Network, TCP, and Reverse Proxy Internals

**7. Q: Reverse Proxy aur Forward Proxy me kya farq hai? Tumne kaunsa banaya hai?**
* **A:** Forward Proxy client ko internet pe access deta hai (client hide hota hai internet se). Maine Reverse Proxy banaya hai jo internet (clients) se aane wali traffic receive karke internal private servers (backends) ko bhejta hai. Backend hide hota hai internet se.

**8. Q: Jab request Proxy ke through jaati hai, toh Backend ko client ka asli IP kaise pata chalega?**
* **A:** Backend ko hamesha Proxy ka IP hi dikhega TCP connection me (`RemoteAddr`). Client ka asli IP batane ke liye Proxy `X-Forwarded-For` header inject karta hai. Yeh standard tarika hai HTTP layer pe real client IP pass karne ka.

**9. Q: Tumne `httputil.ReverseProxy` use kiya. Iske andar `Transport` object ka role kya hai?**
* **A:** `Transport` connection pooling, TCP dial configuration, timeouts, aur TLS handshakes manage karta hai. Without custom Transport, default Go settings use hongi jo high scale par file descriptor leaks (sockets hanging) kar sakti hain.

**10. Q: `Transport.MaxIdleConns` ka kya matlab hai aur load balancer me iski kya importance hai?**
* **A:** Ye batata hai ki proxy kitne TCP connections (sockets) open rakh sakti hai idle state me backends ke saath. TCP handshake (SYN, SYN-ACK, ACK) me latency hoti hai. Connections idle rakhne se agli HTTP request ko purana open socket mil jata hai (Keep-Alive), jisse response time drastic drop hota hai.

**11. Q: TCP Keep-Alive aur HTTP Keep-Alive me kya farq hai?**
* **A:** TCP Keep-Alive OS level par packets (empty ACK) bhej kar check karta hai ki connection zinda hai ya nahi. HTTP Keep-Alive (Connection: keep-alive) application layer par bolta hai "is TCP connection ko close mat karna, main aur HTTP requests bhejunga isi channel pe."

**12. Q: Thundering Herd problem kya hoti hai aur tumhara proxy isse vulnerable hai ya nahi?**
* **A:** Agar saare backends dead ho jayein, aur fir achanak ek backend zinda ho, toh thousands of queued requests us ek backend pe ek saath toot padengi, jisse wo wapas crash ho jayega. Mera proxy isse bachane ke liye koi rate limiting nahi kar raha abhi, so yes, its vulnerable. Isko rokhne ke liye Exponential Backoff ya Circuit Breaker pattern lagana padega.

**13. Q: 502 Bad Gateway aur 504 Gateway Timeout me kya farq hai? Tum kab konsa doge?**
* **A:** 502 Bad Gateway tab hota hai jab Proxy Backend se connect hi na kar paye (Connection Refused). 504 Gateway Timeout tab hota hai jab connect toh ho gaya, request bhi bhej di, par backend ne expected time me response nahi diya (Backend hanging).

**14. Q: Kya tumhara proxy HTTP/2 support karta hai?**
* **A:** Go ka `httputil.ReverseProxy` aur `http.Server` by default HTTP/2 support karte hain agar TLS (HTTPS) configure kiya gaya ho. Abhi main plaintext HTTP/1.1 run kar raha hoon.

**15. Q: WebSockets tumhari proxy ke through kaam karenge? Kaise?**
* **A:** Go ka `httputil.ReverseProxy` `Connection: Upgrade` headers aur `101 Switching Protocols` response ko handle karta hai automatically, TCP connection ka control hijack karke direct I/O pipe client aur backend ke beech open kar deta hai. So yes, WebSockets technically work.

**16. Q: OS limits error `too many open files` proxy server pe kyun aati hai?**
* **A:** Linux me har open socket ek file descriptor hota hai. Agar concurrent connections OS ki max file limit (e.g., `ulimit -n 1024`) cross kar dein, toh naye connections accept ya dial nahi honge. Ise fix karne ke liye system limit badhani padti hai (`ulimit -n 65535`).

---

## Part 3: Architecture & Health Checking

**17. Q: Active Health Check vs Passive Health Check? Tumne dono kyun lagaye?**
* **A:** Active (pinging /health every 5s) backend ko pool me wapas laane ke liye zaroori hai. Passive (failing request immediately evicts backend) tab zaroori hai jab 5s interval ke beech backend gir jaye. Agar passive na ho, toh 5 seconds tak proxy dead server pe request bhejta rahega aur clients ko error milti rahegi. Dono sath me High Availability ensure karte hain.

**18. Q: Agar mera health check endpoint `/health` DB call kar raha ho, kya problem hogi?**
* **A:** Health check bohot frequently hit hota hai. Agar wo database connection involve karta hai, toh load balancer khud database ko DDoS kar dega. Isliye Liveness checks (main zinda hoon) lightweight hone chahiye, jabki Readiness checks (main DB se connected hoon) me cautious hona chahiye aur unki query cached honi chahiye.

**19. Q: Tumhara Health Checker har backend pe parallel goroutine launch karta hai. Agar mere paas 10,000 backends hon toh kya hoga?**
* **A:** 10,000 parallel goroutines health check karne ke liye launch ho jayengi. Go 10k goroutines handle kar lega, but network congestion aur memory spike hoga. Scale par worker pool pattern use karna chahiye health checks ke liye jahan fixed workers queue me se health check tasks uthate hain.

**20. Q: Graceful Shutdown kaise implement kiya tumne? Uske peeche ka mechanism samjhao.**
* **A:** OS signal (`SIGINT/TERM`) aane par maine `server.Shutdown(ctx)` call kiya hai. Ye naye connections accept karna band kar deta hai aur active connections ke poore hone ka wait karta hai timeout tak. Background me `context` use kiya hai jisse agar timeout expire ho toh forced kill ho jaye.

**21. Q: Agar `server.Shutdown()` block ho jaye aur ek client connection intentionally slow data bhej raha ho (Slowloris attack), toh server kabhi band nahi hoga?**
* **A:** Isiliye maine `context.WithTimeout(ctx, 10*time.Second)` use kiya hai `Shutdown` call me. 10 second baad context cancel hoga aur Go runtime bache hue sockets forcefully close kar dega.

**22. Q: Round-Robin ke alawa aur kaunse Load Balancing algorithms hote hain aur unke use cases kya hain?**
* **A:** 
   - *Least Connections*: Jo sabse free ho use request do. Best for long-lived connections (WebSockets).
   - *IP Hash*: Client IP ka hash nikal kar server assign karo. Session Stickiness chahiye tab use hota hai (user ko hamesha same server pe bhejta hai).
   - *Weighted Round Robin*: Powerful servers ko zayda traffic dene ke liye.

**23. Q: Tumhare proxy ka memory footprint kya hai jab wo proxy karta hai? Kya wo poori file memory me load karega agar user 1GB file download kar raha hai?**
* **A:** Nahi. `httputil.ReverseProxy` response body ko memory me load nahi karta, wo `io.CopyBuffer` use karke TCP connection se padhta hai aur turant client TCP connection pe stream kar deta hai chunk-by-chunk. Memory buffer chota aur constant rehta hai.

**24. Q: Tumhare architecture me Load Balancer ek "Leaf Package" hai. Circular dependency avoid karne ka ye pattern kyun use kiya?**
* **A:** Go circular dependencies allow nahi karta (Package A imports B, B imports A). Agar `loadbalancer` package `proxy` ko janta hota, toh architecture tightly coupled ho jata. Leaf package hone ka matlab hai `loadbalancer` purely data structures aur maths own karta hai, jisse main usko akele unit test kar sakta hoon bina proxy start kiye.

---

## Part 4: Edge Cases & Deep Dive Scenarios

**25. Q: Client request aati hai Gateway pe `HTTP/1.0` ke sath. Kya Go Reverse Proxy usey `HTTP/1.1` me convert karega?**
* **A:** Ha, `Transport` object usually upstream server ko `HTTP/1.1` request bhejta hai aur connection reuse karta hai, chahe client ne `HTTP/1.0` kyun na bheja ho.

**26. Q: Agar ek backend slow hai aur har request par 15 second lag raha hai, kya wo active health check pass karega?**
* **A:** Ha, agar health check HTTP client me timeout set nahi hai. Par maine `http.Client` me explicit timeout (e.g., `2s`) lagaya hai. Agar server 2 second me `/health` ka 200 OK nahi deta, toh wo health check dead mark hoga aur traffic roki jayegi.

**27. Q: Tumne passive error handling me `ErrorHandler` me closure capture use kiya hai `b.SetAlive(false)`. Agar backend object delete ho jaye pool se, to us pointer ka kya hoga?**
* **A:** Kyunki pointer goroutine closure me capture ho rakha hai, GC usko destroy nahi karega jab tak closure (proxy function) exist karta hai. Pointer valid rahega. But technically mere static pool se backend objects kabhi delete nahi hote lifecycle me.

**28. Q: Agar TLS termination proxy pe karni ho toh kya changes karne honge code me?**
* **A:** Gateway `http.ListenAndServe` ki jagah `http.ListenAndServeTLS` use karega, jisme TLS cert aur key file path pass karne honge. Client aur Proxy ka connection encrypted hoga, aur Proxy se Backend ka connection internal subnet me plaintext ho sakta hai.

**29. Q: Tumne test cases me `go test -race` specify kiya hai. Data Race kab aati hai Go me?**
* **A:** Jab 2 ya 2 se zyada goroutines same memory location ko access karti hain bina kisi synchronization lock (Mutex ya Atomic) ke, aur unme se kam se kam ek goroutine likh (write) rahi hoti hai. Ye random memory corruption aur bugs create karta hai.

**30. Q: C++ me race condition aur Go me data race me kya farq hai?**
* **A:** Go ka scheduler user-space me run hota hai aur OS threads pe multiplex hota hai. C++ direct OS threads use karta hai. Go runtime khud race detector module inject karta hai build flag `-race` lagane par jo memory shadows ko track karta hai. C++ me TSAN use karna padta hai. Concept same hai.

**31. Q: Kya `context.Background()` goroutine ko forcefully rok deta hai?**
* **A:** Nahi, Go me goroutines ko bahar se "kill" karne ka koi tarika nahi hai. Goroutines ko andar se `ctx.Done()` channel ko sunna padta hai (cooperative cancellation). Agar goroutine select block me context channel nahi check karti, to wo exit nahi hogi leak ho jayegi.

**32. Q: Tumhari Gateway `ServeHTTP` request me if multiple clients start sending huge headers (1MB+), proxy safe hai?**
* **A:** By default, Go's `http.Server` request header ko 1MB tak allow karta hai (`MaxHeaderBytes`). Agar usse zyada aaye toh wo khud connection drop kar dega. Proxy is safe from simple header overflow buffer attacks out of the box.

---

*(Here are extra quick-fire concepts they grill on)*

**33. Context Switching Overhead**: Go uses M:N scheduling. Context switch in Go (goroutine to goroutine) happens in user space (few nanoseconds) vs OS thread context switch which involves kernel space (microseconds).
**34. Garbage Collection Pause**: Go uses a concurrent mark-and-sweep GC. Pauses are sub-millisecond, par proxy me huge memory allocation GC cycle badha sakta hai. Pool techniques (`sync.Pool`) use karni chahiye buffer memory ke liye.
**35. Zero-Copy**: In Linux, `sendfile` bypasses user-space. Reverse proxies rarely use pure zero copy because they have to parse headers at L7, memory buffering is required.
**36. HTTP Host Header**: When proxy forwards request, Host header badalna zaroori hai backend ke host me, warna backend reject kar dega agar usme virtual hosting setup hai.
**37. X-Real-IP vs X-Forwarded-For**: XFF chain of proxies dikhata hai (Client -> Proxy1 -> Proxy2). X-Real-IP mostly immediate client connection ka IP hold karta hai.
**38. Rate Limiting**: Isme Token Bucket algorithm commonly lagta hai. Is project me uski zaroorat padegi agar public facing API gateway banaye.
**39. Sticky Sessions**: Cache hit ratios badhane ke liye load balancer ek user ko hamesha same server pe bhejta hai. Cookie-based ya IP hash se hota hai.
**40. Connection Draining**: Jab shutdown signal aaye, naye connection na lena but purane finish hone dena taaki client ko abrupt socket reset (`RST`) na mile.
**41. Go Interface vs Duck Typing**: Go interface implicit hai, Python duck typing ki tarah runtime nahi, par compile time pe check hoti hai.
**42. Pointer Escape Analysis**: Agar Go compiler dekhta hai ki variable us function ke bahar zinda rahega (like closure me `Backend` struct), toh usko stack ki jagah Heap me daal deta hai jisse GC ko kaam karna padta hai.
**43. Mutex vs Channel for state sharing**: Go kehta hai "Share memory by communicating". Par ek simple variable (`Alive`) jiske liye hazaaron goroutines hit karengi, wahan Mutex/RWMutex/Atomic faster hota hai channel ke context switch se.
**44. Timeouts everywhere**: `DialTimeout`, `TLSHandshakeTimeout`, `IdleConnTimeout`. Bina inke, network partition hone pe proxy hamesha ke liye block hoke sockets exhaust kar degi.
**45. Bounded Queues**: Infinite channels in Go cause OOM.
**46. Circuit Breaker**: Thundering herd aur cascade failure rokne ke liye proxy level par circuit breaker (half-open, open state) implement karna zaruri hota hai.
**47. HTTP Chunked Transfer Encoding**: Go's proxy handles this automatically, memory buffer overflow se bachata hai.
**48. SNI (Server Name Indication)**: TLS handshake me bata hai ki kis domain ke liye connection hai.
**49. 100-Continue**: Client proxy se poochta hai ki main 2GB ka payload bhejoon ya pehle hi reject kar doge? Go proxy `ExpectContinueTimeout` se handle karta hai.
**50. OSI Model L4 vs L7 Proxy**: L4 (TCP Proxy) payload padhe bina byte stream aage fenkta hai (very fast). L7 (yeh project) HTTP request padh kar decide karta hai kahan bhejni hai (slower but intelligent).

---
**Advice:** Inme se questions definitely aayenge if they see a Go Reverse Proxy on your resume. Unko batao tumhe pata hai standard library me kya limits hain aur real world production networking kaise kaam karti hai!
