package client

import (
	"io"
	"sync"

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
