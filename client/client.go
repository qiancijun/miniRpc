package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qiancijun/minirpc/codec"
	"github.com/qiancijun/minirpc/common"
	"github.com/qiancijun/minirpc/errs"
)

type Client struct {
	cc       codec.Codec
	opt      *common.Option
	sending  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

type clientResult struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *common.Option) (client *Client, err error)

func NewClient(conn net.Conn, opt *common.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error: ", err)
		return nil, err
	}

	// 发送 Option 协商
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}

	return newClientCodec(f(conn), opt), nil
}

func NewHTTPClient(conn net.Conn, opt *common.Option) (*Client, error) {
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", common.DefaultRPCPath))

	resq, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{
		Method: "CONNECT",
	})
	if err != nil || resq.Status != common.Connected {
		return nil, errs.ErrUnexpextedHTTPResponse
	}
	cli, err := NewClient(conn, opt)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

// func Dial(network, address string, opts ...*common.Option) (client *Client, err error) {
// 	opt, err := parseOptions(opts...)
// 	if err != nil {
// 		return nil, err
// 	}
// 	conn, err := net.Dial(network, address)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer func() {
// 		if client == nil {
// 			_ = conn.Close()
// 		}
// 	}()
// 	return NewClient(conn, opt)
// }

func Dial(network, address string, opts ...*common.Option) (*Client, error) {
	return dialTimeout(NewClient, network, address, opts...)
}

func DialHTTP(network, address string, opts ...*common.Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

func XDial(rpcAddr string, opts ...*common.Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("rpc client err; wrong format '%s', expect protocal@addr", rpcAddr)
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}

func newClientCodec(cc codec.Codec, opt *common.Option) *Client {
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

func dialTimeout(f newClientFunc, network, address string, opts ...*common.Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	ch := make(chan clientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- clientResult{
			client: client,
			err:    err,
		}
	}()
	if opt.ConnectTimeout == 0 {
		result := <-ch
		return result.client, result.err
	}
	select {
	case <-time.After(opt.ConnectTimeout):
		return nil, errs.ErrClientConnectTimeout
	case result := <-ch:
		return result.client, result.err
	}
}

func parseOptions(opts ...*common.Option) (*common.Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return common.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errs.ErrOptionsEmpty
	}
	opt := opts[0]
	opt.MagicNumber = common.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = common.DefaultOption.CodecType
	}
	return opt, nil
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, errs.ErrShutdown
	}
	call.Seq = c.seq
	c.seq++
	c.pending[call.Seq] = call
	return call.Seq, nil
}

func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

func (c *Client) terminateCalls(err error) {
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = c.cc.ReadHeader(&h); err != nil {
			break
		}
		call := c.removeCall(h.Seq)
		switch {
		case call == nil:
			err = c.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf("%s", h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	// 发生错误了，终止所有的 call
	c.terminateCalls(err)
}

func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()

	// 注册调用
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 准备请求头
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = seq
	c.header.Error = ""

	// 发送请求
	if err := c.cc.Write(&c.header, call.Args); err != nil {
		call := c.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}

func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	// call := <-c.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	call := c.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		// 调用超时
		c.removeCall(call.Seq)
		return errs.ErrClientCallTimeout
	case call := <-call.Done:
		return call.Error
	}
}

// Close implements io.Closer.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return errs.ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

var _ io.Closer = (*Client)(nil)
