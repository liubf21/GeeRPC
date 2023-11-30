# GeeRPC

基于Go的RPC框架，学习自[7天用Go从零实现系列](https://geektutu.com/post/geerpc.html)

参照 golang 标准库 net/rpc，实现了服务端以及支持并发的客户端，并且支持选择不同的序列化与反序列化方式；为了防止服务挂死，在其中一些关键部分添加了超时处理机制；支持 TCP、Unix、HTTP 等多种传输协议；支持多种负载均衡模式，还实现了一个简易的服务注册和发现中心。

## RPC
RPC（Remote Procedure Call）是一种远程过程调用的模式，允许在网络上的不同计算机之间进行函数调用，使得分布式系统可以像调用本地函数一样调用远程函数。它的核心思想是隐藏分布式系统的底层通信细节，使开发者能够更方便地进行跨网络的函数调用，提高代码的可读性和开发效率。

核心思想：

1. 客户端-服务器模型：RPC基于客户端-服务器模型，客户端发起调用请求，而服务器响应请求并返回结果。
2. 远程过程调用：RPC使得开发者可以调用在远程服务器上的函数，就像调用本地函数一样。客户端调用远程函数时，封装函数参数并通过网络传输到服务器端，服务器接收到请求后执行函数调用，并将结果返回给客户端。
3. 序列化和反序列化：由于数据需要在网络上进行传输，RPC需要将函数参数和返回结果序列化（即转换为二进制或其他网络可传输的格式），以便在网络上进行传输。在接收端，同样需要进行反序列化（即将接收到的二进制数据转换为可用的数据对象）。

实现方法：

RPC的实现方法通常包括以下关键组件和步骤：

1. 通信协议：选择合适的通信协议用于在客户端和服务器之间进行通信。常见的选择包括TCP/IP、HTTP、WebSocket等。
2. 序列化和反序列化：选择适当的序列化和反序列化机制，以便在网络上传输函数调用的参数和结果。常见的序列化格式有JSON、XML、Protocol Buffers等。
3. 接口定义：定义客户端和服务器之间的接口，明确函数的名称、参数和返回值，以便客户端可以正确构造请求，服务器可以正确解析请求并发送响应。
4. 代理和存根：客户端和服务器通过代理和存根进行通信。客户端的代理对象封装了远程调用的细节，使得调用看起来像是本地函数调用。服务器的存根对象负责接收请求，并将请求路由到相应的函数执行。
5. 远程调用执行：服务器接收到请求后，根据请求中的函数名称，找到对应的函数，并将参数传递给该函数进行执行。执行完成后，服务器将结果返回给客户端。
6. 客户端回调处理：在某些情况下，客户端可能需要处理回调函数。例如，在异步调用中，客户端发送请求后可以继续执行其他操作，并在将来接收到服务器的响应后触发回调函数进行处理。
7. 错误处理和异常传递：RPC需要正确处理网络通信中的错误和异常情况，并将错误信息传递给调用方。服务器可以抛出异常，并将异常信息序列化后返回给客户端。

实现RPC的具体细节和步骤可能因使用的编程语言、框架和库而有所不同。许多语言和框架都提供了方便的工具和库来简化RPC的实现，如Go语言的标准库中的net/rpc包、Java语言的Java RMI、Python语言的Pyro等。

## RPC框架的实现

传输协议 报文编码 连接超时 异步请求和并发

注册中心(registry)和负载均衡(load balance)

客户端和服务端互相不感知对方的存在，服务端启动时将自己注册到注册中心，客户端调用时，从注册中心获取到所有可用的实例，选择一个来调用。

注册中心通常还需要实现服务动态添加、删除，使用心跳确保服务处于可用状态等功能。

一个典型的 RPC 调用：err = client.Call("Arith.Multiply", args, &reply)
客户端发送的请求包括服务名 Arith，方法名 Multiply，参数 args 三个，服务端的响应包括错误 error，返回值 reply 2 个。

### 1.

消息的序列化和反序列化: Header 消息头，Codec对消息体进行编解码的接口，支持gob和json

通信过程: 客户端和服务端协商实现(为了提升性能，一般在报文的最开始会规划固定的字节，来协商相关的信息。比如第1个字节用来表示序列化方式，第2个字节表示压缩方式，第3-6字节表示 header 的长度，7-10 字节表示 body 的长度。)

这里只需要协商消息的编解码方式，放到结构体Option中，采用json编码，后续的 header 和 body 的编码方式由 Option 中的 CodeType 指定

```
| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
```


服务端的实现: serveCodec 包含三个阶段 readRequest handleRequest sendResponse

main函数

实现了一个消息的编解码器 GobCodec，并且客户端与服务端实现了简单的协议交换(protocol exchange)，即允许客户端使用不同的编码方式。同时实现了服务端的雏形，建立连接，读取、处理并回复客户端的请求

### 2.

对 `net/rpc`，能被远程调用的函数 `func (t *T) MethodName(argType T1, replyType *T2) error` 封装结构体 Call 来承载一次 RPC 调用所需要的信息，在Call中添加类型为chan* Call的字段Done用于通知

实现Client结构体 核心字段：编解码器，互斥锁，请求消息头，请求编号，未处理完的全部请求，是否可用

功能：接收响应、发送请求

实现一个支持异步和并发的高性能客户端

### 3.

如何将结构体的方法映射为服务？ 硬编码x 使用反射，可以获取某个结构体的所有方法，以及方法的参数和返回值类型

```go
func main() {
	var wg sync.WaitGroup
	typ := reflect.TypeOf(&wg)
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		argv := make([]string, 0, method.Type.NumIn())
		returns := make([]string, 0, method.Type.NumOut())
		// j 从 1 开始，第 0 个入参是 wg 自己。
		for j := 1; j < method.Type.NumIn(); j++ {
			argv = append(argv, method.Type.In(j).Name())
		}
		for j := 0; j < method.Type.NumOut(); j++ {
			returns = append(returns, method.Type.Out(j).Name())
		}
		log.Printf("func (w *%s) %s(%s) %s",
			typ.Elem().Name(),
			method.Name,
			strings.Join(argv, ","),
			strings.Join(returns, ","))
    }
}
```

通过反射实现结构体与服务的映射关系，定义结构体methodType和service，实现 call 方法，能够通过反射值调用方法。

从接收到请求到回复的步骤：第一步，根据入参类型，将请求的 body 反序列化；第二步，调用 service.call，完成方法调用；第三步，将 reply 序列化为字节流，构造响应报文，返回。

补充完善服务端中readRequest和handleRequest方法

### 4.

超时处理 

需要客户端处理超时的地方有：

与服务端建立连接，导致的超时
发送请求到服务端，写报文导致的超时
等待服务端处理时，等待处理导致的超时（比如服务端已挂死，迟迟不响应）
从服务端接收响应时，读报文导致的超时

需要服务端处理超时的地方有：

读取客户端请求报文时，读报文导致的超时
发送响应报文时，写报文导致的超时
调用映射服务的方法时，处理报文导致的超时

项目中添加的超时处理： 1. 客户端创建连接时 2. Client.Call 3. Server.handleRequest

在Option中加入超时设定

实现一个超时处理的外壳 dialTimeout，这个壳将 NewClient 作为入参，在 2 个地方添加了超时处理的机制：1. 将 net.Dial 替换为 net.DialTimeout，如果连接创建超时，将返回错误; 2. 使用子协程执行 NewClient，执行完成后则通过信道 ch 发送结果，如果 time.After() 信道先接收到消息，则说明 NewClient 执行超时，返回错误。

方式： 1. 使用 context.WithTimeout; 2. 使用 time.After() 结合 select+chan 完成

### 5.

使用HTTP 协议的 CONNECT 方法完成协议转换

对 RPC 服务端来，需要做的是将 HTTP 协议转换为 RPC 协议，对客户端来说，需要新增通过 HTTP CONNECT 请求创建连接的逻辑。

通信过程：

客户端向 RPC 服务器发送 CONNECT 请求 `CONNECT 10.0.0.1:9999/_geerpc_ HTTP/1.0`

RPC 服务器返回 HTTP 200 状态码表示连接建立。`HTTP/1.0 200 Connected to Gee RPC`

客户端使用创建好的连接发送 RPC 报文，先发送 Option，再发送 N 个请求报文，服务端处理 RPC 请求并响应。

### 6.

假设有多个服务实例，每个实例提供相同的功能，为了提高整个系统的吞吐量，每个实例部署在不同的机器上。客户端可以选择任意一个实例进行调用，获取想要的结果。那如何选择呢？取决了负载均衡的策略。

+ 随机选择策略 - 从服务列表中随机选择一个。
+ 轮询算法(Round Robin) - 依次调度不同的服务器，每次调度执行 i = (i + 1) mode n。
+ 加权轮询(Weight Round Robin) - 在轮询算法的基础上，为每个服务实例设置一个权重，高性能的机器赋予更高的权重，也可以根据服务实例的当前的负载情况做动态的调整，例如考虑最近5分钟部署服务器的 CPU、内存消耗情况。
+ 哈希/一致性哈希策略 - 依据请求的某些特征，计算一个 hash 值，根据 hash 值将请求发送到对应的机器。一致性 hash 还可以解决服务实例动态添加情况下，调度抖动的问题。一致性哈希的一个典型应用场景是分布式缓存服务。感兴趣可以阅读动手写分布式缓存 - GeeCache第四天 一致性哈希(hash)
+ ...

先实现一个最基础的服务发现模块 Discovery，需要的方法 Refresh、Update、Get、GetAll

实现一个不需要注册中心，服务列表由手工维护的服务发现的结构体：MultiServersDiscovery

实现一个支持负载均衡的客户端 XClient

### 7.

实现一个简单的注册中心，支持服务注册、接收心跳等功能

1. 服务端启动后，向注册中心发送注册消息，注册中心得知该服务已经启动，处于可用状态。一般来说，服务端还需要定期向注册中心发送心跳，证明自己还活着。
2. 客户端向注册中心询问，当前哪天服务是可用的，注册中心将可用的服务列表返回客户端。
3. 客户端根据注册中心得到的服务列表，选择其中一个发起调用。

注册中心的功能还有很多，比如配置的动态同步、通知机制等。比较常用的注册中心有 etcd、zookeeper、consul，一般比较出名的微服务或者 RPC 框架，这些主流的注册中心都是支持的。

定义GeeRegistry结构体，实现方法 putServer、aliveServers，采用HTTP协议提供服务，将有用信息承载在HTTP Header，其中Get通过X-Geerpc-Servers承载，Post通过X-Geerpc-Server承载

实现Heartbeat方法，便于服务启动时定时向注册中心发送心跳

客户端实现基于注册中心的服务发现机制：在xclient中实现GeeRegistryDiscovery，基于MultiServersDiscovery，包括registry存注册中心地址，timeout服务列表的过期时间，lastUpdate最后从注册中心更新服务列表的时间(默认10s过期)
