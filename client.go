package geerpc

import (
	"context"
	"encoding/json"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Call struct {
	Seq           uint64
	ServiceMethod string // format "<service>.<method>"
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call // for rpc client; when call is done, it will be used to notify the application
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc       codec.Codec // for transport
	opt      *Option
	sending  sync.Mutex       // protect following
	header   codec.Header     // request header
	mu       sync.Mutex       // protect following
	seq      uint64           // request seq
	pending  map[uint64]*Call // save the call that is waiting for response
	closing  bool             // user has called Close
	shutdown bool             // server has told us to stop
}

var _ io.Closer = (*Client)(nil) // Client must implement io.Closer interface

func newClientByCodec(cc codec.Codec, opt *Option) *Client {
	client := &Client{
		seq:     1, // seq starts from 1, 0 means invalid call
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive() // receive response
	return client
}

func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: options error:", err)
		return nil, err
	}

	if err := json.NewEncoder(conn).Encode(opt); err != nil { // send option to server
		log.Println("rpc client: options error:", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientByCodec(f(conn), opt), nil
}

var ErrShutdown = fmt.Errorf("connection is shut down")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.closing && !client.shutdown
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil: // call has been terminated
			err = client.cc.ReadBody(nil)
		case h.Error != "": // error from server
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default: // read response body and notify application
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = fmt.Errorf("reading body %s", err)
			}
			call.done()
		}
	}
	// error occurs, terminate calls
	client.terminateCalls(err)
}

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	// register this call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// encode and send request
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq) // remove this call
		// call is not nil, because we have registered it before
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Go invokes the function asynchronously. It returns the Call structure representing the invocation.
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil { // make sure done is not nil
		done = make(chan *Call, 10)
	} else if cap(done) == 0 { // make sure done has buffer
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	go client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done(): // context timeout
		client.removeCall(call.Seq) // remove this call
		return fmt.Errorf("rpc client: call failed: %s", ctx.Err())
	case call := <-call.Done: // call is done
		return call.Error
	}
}

func parseOptions(opts ...*Option) (*Option, error) { // to make Option optional
	if len(opts) == 0 || opts[0] == nil { // use default options
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, fmt.Errorf("number of options is more than 1")
	}
	opt := opts[0]
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType // use default codec type
	}
	return opt, nil
}

type clientResult struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)

func dialTimeout(f newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
	opt, err := parseOptions(opts...) // parse options
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout) // connect to server
	if err != nil {
		return nil, err
	}
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	ch := make(chan clientResult)
	go func() { // start a goroutine to create client
		client, err = f(conn, opt)
		ch <- clientResult{client: client, err: err} // send result to ch
	}()
	if opt.ConnectTimeout == 0 { // no timeout
		result := <-ch // wait for result from goroutine
		return result.client, result.err
	}
	select {
	case <-time.After(opt.ConnectTimeout): // timeout
		return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeout)
	case result := <-ch: // receive result from goroutine
		return result.client, result.err
	}
}

func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	return dialTimeout(NewClient, network, address, opts...)
}
