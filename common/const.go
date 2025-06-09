package common

import "time"

const (
	Connected            = "200 Connected to Mini RPC"
	DefaultRPCPath       = "/_minirpc_"
	DefaultDebugPath     = "/debug/minirpc"
	DefaultTimeout       = time.Minute * 5
	DefaultPath          = "/_minirpc_/registry"
	DefaultUpdateTimeout = time.Second * 10
)
