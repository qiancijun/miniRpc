package server

import (
	"reflect"

	"github.com/qiancijun/minirpc/codec"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}
