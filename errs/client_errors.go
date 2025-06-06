package errs

import "errors"

var (
	ErrShutdown = errors.New("connection is shut down")
	ErrOptionsEmpty = errors.New("number of options is more than 1")
)
