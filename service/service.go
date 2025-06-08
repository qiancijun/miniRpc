package service

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type Service struct {
	Name   string
	Typ    reflect.Type
	Rcvr   reflect.Value
	Method map[string]*MethodType
}

func NewService(Rcvr interface{}) *Service {
	s := new(Service)
	s.Rcvr = reflect.ValueOf(Rcvr)
	s.Name = reflect.Indirect(s.Rcvr).Type().Name()
	s.Typ = reflect.TypeOf(Rcvr)
	if !ast.IsExported(s.Name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.Name)
	}
	s.registerMethods()
	return s
}

func (s *Service) registerMethods() {
	s.Method = make(map[string]*MethodType)
	for i := 0; i < s.Typ.NumMethod(); i++ {
		Method := s.Typ.Method(i)
		mType := Method.Type
		// 不满足一个 RPC 的方法
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.Method[Method.Name] = &MethodType{
			Method:    Method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.Name, Method.Name)
	}
}

func (s *Service) Call(m *MethodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.Method.Func
	returnValues := f.Call([]reflect.Value{s.Rcvr, argv, replyv})
	// 检查 error
	// 第一个返回值是 error 类型
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
