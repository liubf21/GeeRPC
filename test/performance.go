package main

import (
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"
)

// Echo 服务端的结构体
type Echo int

// Hi 是 RPC 方法
func (t *Echo) Hi(args string, reply *string) error {
	*reply = "Echo: " + args
	return nil
}

// startServer 启动 RPC 服务器
func startServer(wg *sync.WaitGroup) {
	defer wg.Done()

	echo := new(Echo)
	rpc.Register(echo)

	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("ListenTCP error:", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			break
		}
		go rpc.ServeConn(conn)
	}
}

// testClient 是客户端测试函数
func testClient(wg *sync.WaitGroup) {
	defer wg.Done()

	client, err := rpc.Dial("tcp", "localhost:1234")
	if err != nil {
		log.Fatal("Dialing:", err)
	}

	start := time.Now()

	for i := 0; i < 1000; i++ {
		var reply string
		err = client.Call("Echo.Hi", "Hello", &reply)
		if err != nil {
			log.Fatal("rpc error:", err)
		}
		// log.Println(reply)
	}

	elapsed := time.Since(start)
	log.Printf("1000 calls took %s", elapsed)
}

func main() {
	var wg sync.WaitGroup

	wg.Add(1)
	go startServer(&wg)

	// 等待服务器启动
	time.Sleep(time.Second)

	wg.Add(1)
	go testClient(&wg)

	wg.Wait()
}
