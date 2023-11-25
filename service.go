package geerpc

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint64 // count method call
}

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls) // atomic load
}

func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr { // if arg is pointer
		argv = reflect.New(m.ArgType.Elem()) // create pointer
	} else {
		argv = reflect.New(m.ArgType).Elem() // create value
	}
	return argv
}

func (m *methodType) newReplyv() reflect.Value {
	replyv := reflect.New(m.ReplyType.Elem()) // create pointer
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem())) // create map
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0)) // create slice
	}
	return replyv
}

type service struct {
	name   string
	typ    reflect.Type
	rcvr   reflect.Value          // save receiver of methods
	method map[string]*methodType // save methods of this service
}

func newService(rcvr interface{}) *service {
	s := new(service)
	s.rcvr = reflect.ValueOf(rcvr) // get receiver of methods
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)
	log.Printf("rpc service: new service %s; type: %s\n", s.name, s.typ.String())
	s.registerMethods()
	return s
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == "" // check if type is exported or builtin
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	log.Printf("rpc service: register %s\n", s.name)
	for i := 0; i < s.typ.NumMethod(); i++ { // iterate all methods of this service
		log.Printf("rpc service: register %s.%s\n", s.name, s.typ.Method(i).Name)
		method := s.typ.Method(i)
		mType := method.Type
		if mType.NumIn() != 3 || mType.NumOut() != 1 { // check method signature: (receiver, *args, *reply) error
			log.Printf("method %s has wrong number of ins or outs: %d, %d\n", method.Name, mType.NumIn(), mType.NumOut())
			continue
		}
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() { // check return type
			log.Printf("method %s returns %s, not error\n", method.Name, mType.Out(0))
			continue
		}
		argType, replyType := mType.In(1), mType.In(2) // check arg type and reply type
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			log.Printf("method %s argument or reply type not exported: %v %v\n", method.Name, argType, replyType)
			continue
		}
		s.method[method.Name] = &methodType{ // register method
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc service: register %s.%s\n", s.name, method.Name)
	}
}

func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1) // count method call by 1
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv}) // call method
	if errInter := returnValues[0].Interface(); errInter != nil { // get error
		return errInter.(error)
	}
	return nil
}
