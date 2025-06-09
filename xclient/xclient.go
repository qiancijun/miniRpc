package xclient

import (
	"context"
	"io"
	"reflect"
	"sync"

	"github.com/qiancijun/minirpc/client"
	"github.com/qiancijun/minirpc/common"
)

type XClient struct {
	d       Discovery
	mode    SelectMode
	opt     *common.Option
	mu      sync.Mutex
	clients map[string]*client.Client
}

func NewXClient(d Discovery, mode SelectMode, opt *common.Option) *XClient {
	return &XClient{
		d:       d,
		mode:    mode,
		opt:     opt,
		clients: make(map[string]*client.Client),
	}
}

// Close implements io.Closer.
func (x *XClient) Close() error {
	x.mu.Lock()
	defer x.mu.Unlock()
	for key, client := range x.clients {
		_ = client.Close()
		delete(x.clients, key)
	}
	return nil
}

var _ io.Closer = (*XClient)(nil)

func (xc *XClient) dial(rpcAddr string) (*client.Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	cli, ok := xc.clients[rpcAddr]
	if ok && !cli.IsAvailable() {
		_ = cli.Close()
		delete(xc.clients, rpcAddr)
		cli = nil
	}
	if cli == nil {
		var err error
		cli, err = client.XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = cli
	}
	return cli, nil
}

func (xc *XClient) call(ctx context.Context, rpcAddr string, serviceMethod string, args, reply interface{}) error {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

// Call invokes the named function, waits for it to complete,
// and returns its error status.
// xc will choose a proper server.
func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(ctx, rpcAddr, serviceMethod, args, reply)
}

func (x *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := x.d.GetAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error

	replyDone := reply == nil
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := x.call(ctx, rpcAddr, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel()
			}
			if err == nil && !replyDone {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}
