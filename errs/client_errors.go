package errs

import "errors"

var (
	ErrShutdown = errors.New("connection is shut down")
)
