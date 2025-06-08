package server

import (
	"reflect"

	"github.com/qiancijun/minirpc/codec"
	"github.com/qiancijun/minirpc/service"
)

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
	mtype        *service.MethodType
	svc          *service.Service
}
