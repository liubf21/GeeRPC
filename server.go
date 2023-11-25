package geerpc

import (
	"encoding/json"
	"errors"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
)

const MagicNumber = 0x3bef5c // magic number identifies rpc request

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

type Server struct {
	serviceMap sync.Map // use sync.Map to store service name and its corresponding service
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Register(rcvr interface{}) error {
	svc := newService(rcvr)                                     // create service
	if _, dup := s.serviceMap.LoadOrStore(svc.name, svc); dup { // check if service is already registered
		return errors.New("rpc: service already defined: " + svc.name)
	}
	return nil
}

func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".") // find the last index of '.'
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:] // get service name and method name
	svci, ok := s.serviceMap.Load(serviceName)                            // get service from service map
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*service) // type assertion
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

// Accept accepts connections on the listener and serves requests for each incoming connection.
func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept() // wait for a connection request
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go s.ServerConn(conn)
	}
}

func (s *Server) ServerConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil { // decode option
		log.Println("rpc server: options error:", err)
		return
	}
	if opt.MagicNumber != MagicNumber { // check magic number
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType] // get corresponding codec constructor
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	s.serveCodec(f(conn))
}

var invalidRequest = struct{}{}

func (s *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex) // make sure to send a complete response
	wg := new(sync.WaitGroup)  // wait until all request are handled
	for {
		req, err := s.readRequest(cc) // read request
		if err != nil {
			if req == nil {
				break // it's not possible to recover, so close the connection
			}
			req.h.Error = err.Error() // encode error message in response header
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg) // handle request concurrently
	}
	wg.Wait()
	_ = cc.Close()
}

// request stores all information of a call
type request struct {
	h            *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
	mtype        *methodType   // method type of request
	svc          *service      // service of request
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil { // read request header
		if err != io.EOF && err != io.ErrUnexpectedEOF { // io.EOF means end of connection
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(cc) // read request header
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.svc, req.mtype, err = s.findService(h.ServiceMethod) // find service and method type
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()     // create argv
	req.replyv = req.mtype.newReplyv() // create replyv

	// make sure argv is a pointer, read request body will decode into argv
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = cc.ReadBody(argvi); err != nil { // read request body
		log.Println("rpc server: read body error:", err)
		return req, err
	}
	return req, nil
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock() // make sure to send a complete response
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil { // encode and send response
		log.Println("rpc server: write response error:", err)
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("rpc server: receive request:", req.h, req.argv)
	err := req.svc.call(req.mtype, req.argv, req.replyv) // call service method
	if err != nil {
		req.h.Error = err.Error()
		s.sendResponse(cc, req.h, invalidRequest, sending)
		return
	}
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

var DefaultServer = NewServer()

func Accept(lis net.Listener) { DefaultServer.Accept(lis) }
