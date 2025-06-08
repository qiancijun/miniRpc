package client

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/qiancijun/minirpc/common"
	"github.com/qiancijun/minirpc/errs"
	"github.com/qiancijun/minirpc/server"
	"github.com/stretchr/testify/assert"
)

func TestClient_dialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":0")

	f := func(conn net.Conn, opt *common.Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(time.Second * 2)
		return nil, nil
	}
	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &common.Option{ConnectTimeout: time.Second})
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), errs.ErrClientConnectTimeout.Error())
	})
	t.Run("0", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &common.Option{ConnectTimeout: 0})
		assert.NoError(t, err)
	})
}

type Bar int

func (b Bar) Timeout(argv int, reply *int) error {
	time.Sleep(time.Second * 2)
	return nil
}

func startServer(addr chan string) {
	var b Bar
	_ = server.Register(&b)
	// pick a free port
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	opts := server.ServerOption{Timeout: 1 * time.Second}
	server.Accept(l, opts)
}

func TestClient_Call(t *testing.T) {
	t.Parallel()
	addrCh := make(chan string)
	go startServer(addrCh)
	addr := <-addrCh
	time.Sleep(time.Second)
	t.Run("client timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var reply int
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), errs.ErrClientCallTimeout.Error())
	})
	t.Run("server handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", addr, &common.Option{
			HandleTimeout: time.Second,
		})
		var reply int
		err := client.Call(context.Background(), "Bar.Timeout", 1, &reply)
		assert.NotNil(t, err)
		assert.Equal(t, errs.ErrServiceHandleTimeout.Error(), err.Error())
	})
}
