package client

type Call struct {
	Seq           uint64
	ServiceMethod string // 形如 <service>.<method>
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

func (call *Call) done() {
	call.Done <- call
}
