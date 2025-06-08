package errs

import "errors"

var (
	ErrServiceAlreadyDefined = errors.New("rpc: service already defined")
	ErrServiceIllFormed = errors.New("rpc server: service/method request ill-formed")
	ErrServiceNotFound = errors.New("rpc server: can't find service")
	ErrServiceHandleTimeout = errors.New("rpc server: request handle timeout")
)