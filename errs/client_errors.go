package errs

import "errors"

var (
	ErrShutdown = errors.New("connection is shut down")
	ErrOptionsEmpty = errors.New("number of options is more than 1")
	ErrClientConnectTimeout = errors.New("rpc client: connect timeout")
	ErrClientCallTimeout = errors.New("rpc client: call timeout")
)
