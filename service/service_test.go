package service

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Foo int

type Args struct{ Num1, Num2 int }

func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f Foo) sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func TestNewService(t *testing.T) {
	var foo Foo
	s := NewService(&foo)
	assert.Equal(t, len(s.Method), 1)
	mType := s.Method["Sum"]
	assert.NotNil(t, mType)
}

func TestMethodTypeCall(t *testing.T) {
	var foo Foo
	s := NewService(&foo)
	mType := s.Method["Sum"]

	argv := mType.NewArgv()
	replyv := mType.NewReplyv()
	argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 3}))
	err := s.Call(mType, argv, replyv)
	assert.NoError(t, err)
	assert.Equal(t, *replyv.Interface().(*int), 4)
	assert.Equal(t, mType.NumCalls(), uint64(1))
}