package errs

import "errors"

var (
	ErrNoAvailableServers = errors.New("rpc discovery: no available servers")
	ErrNotSupportedSelectMode = errors.New("rpc discovery: not supported select mode")
)