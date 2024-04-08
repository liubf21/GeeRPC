# GeeRPC

[TOC]

基于Go的RPC框架，学习自[7天用Go从零实现系列](https://geektutu.com/post/geerpc.html)

参照 golang 标准库 net/rpc，实现了服务端以及支持并发的客户端，并且支持选择不同的序列化与反序列化方式；为了防止服务挂死，在其中一些关键部分添加了超时处理机制；支持 TCP、Unix、HTTP 等多种传输协议；支持多种负载均衡模式，还实现了一个简易的服务注册和发现中心。

TODO:
- [ ] 基本功能实现
  - [x] 服务端
  - [x] 客户端
  - [x] 服务注册
  - [x] 超时处理
  - [x] HTTP协议
  - [x] 负载均衡
  - [x] 服务发现和注册中心
- [ ] ProtoBuf 支持
- [ ] 负载均衡策略增加：加权轮询、一致性哈希
- [ ] 测试 `net/rpc` 包
- [ ] 跨语言调用测试
- [ ] 问题：实现部分并不直观，对入门者很难想到为什么要这么设计和抽象

## RPC 概念

RPC（Remote Procedure Call）是一种远程过程调用的模式，允许在网络上的不同计算机之间进行函数调用，使得分布式系统可以像调用本地函数一样调用远程函数。它的核心思想是隐藏分布式系统的底层通信细节，使开发者能够更方便地进行跨网络的函数调用，提高代码的可读性和开发效率。

核心思想：

1. 客户端-服务器模型：RPC遵循客户端发起请求，服务器端响应并返回结果的模式。
2. 透明性：RPC隐藏了网络通信的复杂性，使得开发者能够专注于业务逻辑。
3. 序列化与反序列化：为了网络传输，RPC需要将数据转换为可传输的格式（序列化），并在接收端将其还原（反序列化）。

### 为什么需要RPC

[既然有 HTTP 协议，为什么还要有 RPC？](https://xiaolincoding.com/network/2_http/http_rpc.html)

RPC（远程过程调用）作为一种通信机制，在现代软件开发中扮演着重要角色。以下是RPC的几个关键优势，解释了为什么我们需要它：

#### 1. 实现微服务拆分
在传统的单体架构中，所有的功能都集成在一个大型的应用程序中。随着应用的增长，这种架构会导致代码复杂度高、部署困难和迭代缓慢。RPC支持将单体应用拆分为多个独立的微服务，每个服务负责特定的业务功能。这样，团队可以并行开发和部署不同的服务，提高了迭代效率和系统的可维护性。

#### 2. 实现跨语言调用
在多语言的软件开发环境中，RPC提供了一种机制，允许不同编程语言编写的服务之间进行通信。这意味着开发者可以根据需求和团队专长选择合适的编程语言来开发服务，同时还能保持整个系统的协同工作。RPC框架通常提供了多种语言的客户端库，简化了跨语言调用的复杂性。

#### 3. 实现分布式系统
分布式系统通过在多个计算机上分布计算任务来提高性能和可靠性。RPC框架使得在分布式环境中的服务调用变得简单，就像调用本地方法一样。它隐藏了网络通信的细节，使得开发者可以专注于业务逻辑，而不必担心分布式系统带来的复杂性。此外，RPC框架还支持负载均衡和服务发现，这对于构建可扩展的分布式应用至关重要。

#### 4. 实现异常隔离
在分布式系统中，服务之间的依赖关系可能导致错误传播，影响整个系统的稳定性。RPC框架通常提供了异常隔离机制，确保一个服务的失败不会直接影响到其他服务。通过在客户端和服务端处理异常，RPC框架帮助开发者实现更加健壮和可靠的系统设计。

总结来说，RPC通过简化微服务架构的实现、支持跨语言调用、提供分布式系统的通信基础，以及实现异常隔离，成为了现代软件开发中不可或缺的工具。它使得开发者能够构建更加灵活、可扩展和可靠的系统。

 ### Go语言中的`net/rpc`包

Go语言的`net/rpc`包是一个内置的RPC实现，它提供了一套简单的API来实现远程过程调用。这个包的特点和使用方法如下：

#### 简单易用的API
- `net/rpc`包的API设计简洁，易于学习和使用。它允许开发者快速构建客户端和服务器端的RPC通信。

#### 基于接口的服务定义
- 服务端需要实现一个接口，该接口定义了RPC服务的方法。客户端通过这个接口进行远程调用。

#### 并发处理能力
- Go语言的并发模型（goroutines）被`net/rpc`包自然地集成，使得服务端能够处理多个并发请求。

### 服务定义和注册
- **服务对象**: RPC服务通过定义一个Go类型来创建，该类型包含特定签名的方法。
  - 方法必须是公开的，接受两个参数（第一个是参数，第二个是结果，且第二个参数必须是指针），并返回一个`error`。
- **注册服务**: 使用`rpc.Register`函数将服务实例注册到RPC服务器。

```go
type Args struct {
    A, B int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
    *reply = args.A * args.B
    return nil
}

func main() {
    arith := new(Arith)
    rpc.Register(arith)
}
```

### 网络监听和服务处理
- **支持的协议**: `net/rpc`支持多种网络协议，包括TCP和HTTP。
- **TCP服务器**: 通过`net.Listen`创建TCP监听器，并使用`rpc.Accept`开始接受客户端连接。
- **HTTP服务器**: 通过`rpc.HandleHTTP`结合HTTP协议提供RPC服务，并使用`http.ListenAndServe`启动HTTP服务器。

TCP服务器示例：
```go
listener, _ := net.Listen("tcp", ":1234")
defer listener.Close()
rpc.Accept(listener)
```

HTTP服务器示例：
```go
rpc.HandleHTTP()
http.ListenAndServe(":1234", nil)
```

### 客户端实现
- **拨号服务器**: 客户端使用`rpc.Dial`函数连接到RPC服务器。
- **同步调用**: 使用`client.Call`方法进行同步远程调用，传递方法名、参数和结果指针。

  ```go
  client, err := rpc.Dial("tcp", "server:1234")
  // 或使用 rpc.DialHTTP("tcp", "server:1234")
  if err != nil {
      log.Fatal("Dialing:", err)
  }

  args := &Args{7, 8}
  var reply int
  err = client.Call("Arith.Multiply", args, &reply)
  ```

### 高级特性
- **并发处理**: 自然支持并发请求处理。
- **自定义编码**: 默认使用Gob编码，但可以通过实现`rpc.ServerCodec`接口来使用其他编码方式，如JSON。
- **超时和取消**: 可以结合`context`包实现调用的超时和取消。
- **错误处理**: 提供了网络错误和调用错误的处理机制。
- **安全性**: 支持通过TLS等方式进行加密通信。
- **服务发现和负载均衡**: 在分布式环境中，可以集成服务发现和负载均衡机制。

需要注意的是，尽管`net/rpc`包功能强大，但它不支持跨语言调用。这意味着客户端和服务器端必须使用相同的编程语言（Go）来实现。

在后续部分，我们可以探讨本项目与Go标准库`net/rpc`包的对比，特别是在性能优化、协议支持、序列化选项等方面的差异和改进。这将帮助读者理解本项目如何提供更现代、更高效的RPC解决方案。


### 服务端示例

首先，我们定义一个服务接口，并实现这个接口来提供远程调用的方法。

```go
package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
)

// 定义服务接口
type Arith int

// 实现接口方法
func (t Arith) Multiply(args *struct{ A, B int }, reply *int) error {
	*reply = args.A * args.B
	return nil
}

func main() {
	// 注册服务
	rpc.Register(Arith(0))

	// 创建TCP监听器
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	fmt.Println("Listening on :1234")

	// 开始接受连接
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		go rpc.ServeConn(conn)
	}
}
```

### 客户端示例

在客户端，我们创建一个连接到服务端的RPC客户端，并调用服务端的方法。

```go
package main

import (
	"fmt"
	"net/rpc"
	"os"
)

func main() {
	// 拨号连接到服务端
	client, err := rpc.Dial("tcp", "localhost:1234")
	if err != nil {
		fmt.Println("Error dialing:", err.Error())
		os.Exit(1)
	}

	// 创建请求参数
	args := struct{ A, B int }{7, 8}
	var reply int

	// 调用服务端方法
	err = client.Call("Arith.Multiply", &args, &reply)
	if err != nil {
		fmt.Println("Error calling Arith.Multiply:", err.Error())
		os.Exit(1)
	}

	fmt.Printf("Arith.Multiply(7, 8) = %d\n", reply)
}
```

在这两个示例中，我们创建了一个简单的算术服务，它提供了一个乘法操作。服务端监听在`localhost:1234`，客户端连接到这个地址并调用`Multiply`方法。

这些示例展示了`net/rpc`包的基本用法，包括服务定义、注册、监听、拨号和远程调用。对于初学者来说，这些代码可以作为学习和实践RPC的起点。在实际应用中，可能需要处理更复杂的数据结构、错误处理和并发控制。

## RPC 框架实现方法

### 1. 通信协议选择
- **目的**：确定客户端与服务器之间数据交换的协议，这将影响通信的稳定性和性能。
- **选项**：TCP/IP提供可靠的连接，HTTP/HTTPS支持Web集成，WebSocket适用于实时应用。
- **实现**：根据所选协议，实现相应的网络通信层，处理连接建立、数据发送和接收等。

### 2. 序列化与反序列化
- **目的**：为了在网络上传输数据，需要将数据结构转换为字节流。
- **机制**：选择合适的序列化机制，如JSON、XML、Protocol Buffers等，根据性能和兼容性需求做出选择。
- **实现**：开发或集成序列化库，用于在客户端和服务端之间转换数据格式。

### 3. 接口定义
- **目的**：明确服务的契约，包括可用的方法、参数和返回类型。
- **方法**：使用接口定义语言（IDL）或代码生成工具来定义服务接口。
- **实现**：在客户端和服务端生成相应的代码，以实现接口调用和处理。

### 4. 代理与存根
- **目的**：代理和存根是RPC的抽象层，它们隐藏了网络通信的细节。
- **代理**：客户端的代理对象允许开发者像调用本地方法一样调用远程方法。
- **存根**：服务器的存根对象接收网络请求，将其转换为本地方法调用，并处理响应。

### 5. 远程调用执行
- **流程**：服务器接收到请求后，根据请求中的信息调用相应的本地方法，并传递参数。
- **结果**：方法执行后，服务器将结果序列化并发送回客户端。

### 6. 客户端回调处理
- **异步调用**：在异步RPC中，客户端在发送请求后可以继续执行，而不必等待响应。
- **回调机制**：客户端提供一个回调函数，当服务器处理完成并返回结果时，回调函数被触发。

### 7. 错误处理和异常传递
- **异常捕获**：RPC框架应能捕获网络通信过程中的错误和服务器端的异常。
- **错误传递**：将异常信息封装并序列化，然后通过网络传递给客户端，以便进行错误处理。

### 8. 负载均衡与服务发现
- **负载均衡**：在多服务器环境中，通过负载均衡策略分散请求，提高系统的可用性和扩展性。
- **服务发现**：客户端需要一种机制来发现服务实例，这可以通过服务注册中心或DNS服务实现。

### 9. 安全性考虑
- **认证**：确保只有授权的客户端可以调用服务。
- **加密**：对传输的数据进行加密，保护数据的隐私和完整性。

在实现RPC框架时，这些步骤和组件需要综合考虑，以确保框架的健壮性、可扩展性和安全性。开发者可以根据具体的应用场景和需求，选择合适的技术和工具来构建RPC框架。


## GeeRPC 的实现

传输协议 报文编码 连接超时 异步请求和并发

注册中心(registry)和负载均衡(load balance)

客户端和服务端互相不感知对方的存在，服务端启动时将自己注册到注册中心，客户端调用时，从注册中心获取到所有可用的实例，选择一个来调用。

注册中心通常还需要实现服务动态添加、删除，使用心跳确保服务处于可用状态等功能。

一个典型的 RPC 调用：`err = client.Call("Arith.Multiply", args, &reply)`
客户端发送的请求包括服务名 Arith，方法名 Multiply，参数 args 三个，服务端的响应包括错误 error，返回值 reply 2 个。

### 1.

消息的序列化和反序列化: Header 消息头（需要包含服务名、方法名、请求序号、返回的错误信息，将请求和响应中的参数和返回值抽象为 body），Codec 抽象为对消息体进行编解码的接口，目前实现了 gob，之后考虑支持 json 和 protobuf 等

通信过程: 客户端和服务端协商实现

> 客户端与服务端的通信需要协商一些内容，例如 HTTP 报文，分为 header 和 body 2 部分，body 的格式和长度通过 header 中的 Content-Type 和 Content-Length 指定，服务端通过解析 header 就能够知道如何从 body 中读取需要的信息。对于 RPC 协议来说，这部分协商是需要自主设计的。为了提升性能，一般在报文的最开始会规划固定的字节，来协商相关的信息。比如第1个字节用来表示序列化方式，第2个字节表示压缩方式，第3-6字节表示 header 的长度，7-10 字节表示 body 的长度。

这里只需要协商消息的编解码方式，放到结构体Option中，采用json编码，后续的 header 和 body 的编码方式由 Option 中的 CodeType 指定

报文形式如下：
```
| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
```


服务端的实现: 
1. 用 net.Listener 实现 Accept，对每个连接建立一个独立的 goroutine 处理，即 ServeConn
2. 先解码 Option，通过 CodeType 创建对应的编解码器，传入 serveCodec，
3. serveCodec 包含三个阶段 readRequest handleRequest sendResponse
   1. handleRequest 并发处理请求
   2. sendResponse 发送响应，需要加锁，防止多个请求同时写入

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
2. 客户端向注册中心询问，当前哪个服务是可用的，注册中心将可用的服务列表返回客户端。
3. 客户端根据注册中心得到的服务列表，选择其中一个发起调用。

注册中心的功能还有很多，比如配置的动态同步、通知机制等。比较常用的注册中心有 etcd、zookeeper、consul，一般比较出名的微服务或者 RPC 框架，这些主流的注册中心都是支持的。

定义GeeRegistry结构体，实现方法 putServer、aliveServers，采用HTTP协议提供服务，将有用信息承载在HTTP Header，其中Get通过X-Geerpc-Servers承载，Post通过X-Geerpc-Server承载

实现Heartbeat方法，便于服务启动时定时向注册中心发送心跳

客户端实现基于注册中心的服务发现机制：在xclient中实现GeeRegistryDiscovery，基于MultiServersDiscovery，包括registry存注册中心地址，timeout服务列表的过期时间，lastUpdate最后从注册中心更新服务列表的时间(默认10s过期)

## 使用gRPC进行Go和Python之间的RPC调用

gRPC使用Protocol Buffers（protobuf）作为接口描述语言（IDL），允许你定义服务接口和消息格式，然后自动生成对应语言的代码。这样，Go和Python客户端和服务端就可以使用相同的接口定义进行通信。

首先，你需要定义一个`.proto`文件来描述服务接口和消息格式。然后，使用protobuf编译器（`protoc`）为Go和Python生成代码。

### 1. 定义服务接口（`calculator.proto`）:

```protobuf
syntax = "proto3";

// 定义请求和响应消息
message MultiplyRequest {
    int32 a = 1;
    int32 b = 2;
}

message MultiplyResponse {
    int32 result = 1;
}

// 定义服务接口
service CalculatorService {
    rpc Multiply (MultiplyRequest) returns (MultiplyResponse);
}
```

### 2. 生成Go和Python代码:

使用`protoc`编译器为Go和Python生成代码。确保你已经安装了`protoc`和相应的gRPC插件。

```sh
protoc --go_out=. --go-grpc_out=. calculator.proto
protoc --python_out=. calculator.proto
```

这将生成Go和Python的代码，你可以在Go和Python项目中使用这些代码。

### 3. 实现Go服务端:

使用生成的Go代码实现服务端。

```go
package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"./calculatorpb" // 导入生成的Go代码
)

type server struct{}

func (s *server) Multiply(ctx context.Context, req *calculatorpb.MultiplyRequest) (*calculatorpb.MultiplyResponse, error) {
	return &calculatorpb.MultiplyResponse{Result: int32(req.A * req.B)}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	calculatorpb.RegisterCalculatorServiceServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
```

### 4. 实现Python客户端:

使用生成的Python代码实现客户端。

```python
import grpc
from calculatorpb import calculator_service_pb2
from calculatorpb import calculator_service_pb2_grpc

def multiply(a, b):
    with grpc.insecure_channel('localhost:50051') as channel:
        stub = calculator_service_pb2_grpc.CalculatorServiceStub(channel)
        request = calculator_service_pb2.MultiplyRequest(a=a, b=b)
        response = stub.Multiply(request)
        return response.result

print(multiply(3, 4))
```

在这个示例中，我们定义了一个简单的乘法服务。Go服务端实现了这个服务，而Python客户端调用了这个服务。通过gRPC，我们能够在不同语言之间进行标准的RPC调用，而不需要手动处理HTTP请求和响应。这种方法更加符合RPC的设计理念，并且能够提供更好的类型安全和性能。
