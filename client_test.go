package geerpc

import (
	"context"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClient_dialTimeout(t *testing.T) {
	t.Parallel() // mark this test as capable of running in parallel with other tests
	l, _ := net.Listen("tcp", ":0")

	f := func(conn net.Conn, opt *Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(time.Second * 2)
		return nil, nil
	}
	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectTimeout: time.Second})
		_assert(err != nil && strings.Contains(err.Error(), "timeout"), "expect a timeout error")
	})
	t.Run("0", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectTimeout: 0})
		_assert(err == nil, "0 means no limit")
	})
}

type Bar int

func (b Bar) Timeout(argv int, reply *int) error {
	time.Sleep(time.Second * 2)
	return nil
}

func startServer(addr chan string) {
	var b Bar
	_ = Register(&b)
	l, _ := net.Listen("tcp", ":0")
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	Accept(l)
}

func TestClient_Call(t *testing.T) {
	t.Parallel()
	addrCh := make(chan string)
	go startServer(addrCh)

	addr := <-addrCh
	time.Sleep(time.Second)
	t.Run("client timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr)
		defer func() { _ = client.Close() }()
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		var reply int
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), ctx.Err().Error()), "expect a timeout error")
	})
	t.Run("server handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr, &Option{HandleTimeout: time.Second, MagicNumber: MagicNumber}) // if MagicNumber is not set, the client will not send the option struct to the server
		defer func() { _ = client.Close() }()
		var reply int
		err := client.Call(context.Background(), "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), "handle timeout"), "expect a timeout error")
	})
}

func TestXDial(t *testing.T) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" { // darwin is the GOOS for macOS
		ch := make(chan struct{})

		// 创建临时目录
		dir, err := os.MkdirTemp("", "geerpc_test")
		if err != nil {
			t.Fatal("failed to create temp dir", err)
		}
		defer os.RemoveAll(dir) // 清理临时目录

		addr := dir + "/geerpc_test.sock"

		go func() {
			_ = os.Remove(addr)
			l, err := net.Listen("unix", addr)
			if err != nil {
				t.Fatal("failed to listen unix socket", err)
			}
			ch <- struct{}{}
			Accept(l) // 假设这是你的自定义函数
		}()

		<-ch
		_, err = XDial("unix@"+addr, &Option{MagicNumber: MagicNumber}) // 假设XDial是你的自定义函数
		_assert(err == nil, "XDial unix socket error: %v", err)         // 假设_assert是你的自定义断言函数
	}
}
