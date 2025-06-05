package server

import (
	"reflect"

	"github.com/qiancijun/minirpc/codec"
)

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}
