package geerpc

import (
	"fmt"
	"reflect"
	"testing"
)

type Foo int

type Args struct{ Num1, Num2 int }

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) sum(args Args, reply *int) error { // any unexported method will not be registered
	*reply = args.Num1 + args.Num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(s != nil, "service is nil")
	_assert(len(s.method) == 1, "wrong methods len, expect 1, but got %d", len(s.method))
	mType := s.method["Sum"]
	_assert(mType != nil, "method is nil")
	_assert(mType.ArgType == reflect.TypeOf(Args{}), "wrong args type")
	// _assert(mType.ReplyType == reflect.TypeOf(0), "wrong reply type")
}

func TestMethodType_Call(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Sum"]

	argv := mType.newArgv()
	replyv := mType.newReplyv()
	argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 2}))
	err := s.call(mType, argv, replyv)
	_assert(err == nil && *replyv.Interface().(*int) == 3 && mType.NumCalls() == 1, "failed to call Foo.Sum")
}
