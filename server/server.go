package server

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/qiancijun/minirpc/codec"
	"github.com/qiancijun/minirpc/common"
	"github.com/qiancijun/minirpc/errs"
	"github.com/qiancijun/minirpc/service"
)

type Server struct {
	serviceMap sync.Map
}

type ServerOption struct {
	Timeout time.Duration
}

var (
	DefaultServer       = NewServer()
	DefaultServerOption = ServerOption{
		Timeout: 10 * time.Second,
	}
	invalidRequest = struct{}{}
)

func NewServer() *Server {
	return &Server{}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/pain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405  must CONNECT\n")
		return
	}

	conn, _, err := w.(http.Hijacker).Hijack()

	if err != nil {
		log.Print("rpc hijacking ", r.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/1.0 "+common.Connected+"\n\n")
	s.ServeConn(conn, DefaultServerOption)
}

func (s *Server) HandleHTTP() {
	http.Handle(common.DefaultRPCPath, s)
	http.Handle(common.DefaultDebugPath, DebugHTTP{s})
	log.Println("rpc server debug path: ", common.DefaultDebugPath)
}

func (s *Server) Accept(lis net.Listener, opts ServerOption) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error: ", err)
			return
		}
		go s.ServeConn(conn, opts)
	}
}

func (s *Server) ServeConn(conn io.ReadWriteCloser, opts ServerOption) {
	defer func() {
		_ = conn.Close()
	}()

	var opt common.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}

	if opt.MagicNumber != common.MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}

	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	s.serveCodec(f(conn), opts.Timeout)
}

func (s *Server) Register(rcvr interface{}) error {
	svc := service.NewService(rcvr)
	if _, dup := s.serviceMap.LoadOrStore(svc.Name, svc); dup {
		return errs.ErrServiceAlreadyDefined
	}
	return nil
}

func (s *Server) serveCodec(cc codec.Codec, timeout time.Duration) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg, timeout)
	}
	wg.Add(1)
	_ = cc.Close()
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{
		h: h,
	}

	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.NewArgv()
	req.replyv = req.mtype.NewReplyv()

	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}

	if err := cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read argv err: ", err)
		return req, err
	}
	return req, nil
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error: ", err)
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()

	finish := make(chan struct{})
	defer close(finish)
	called := make(chan struct{})
	sent := make(chan struct{})

	go func() {
		err := req.svc.Call(req.mtype, req.argv, req.replyv)
		select {
		case <-finish:
			close(called)
			close(sent)
			return
		case called <- struct{}{}:
			if err != nil {
				req.h.Error = err.Error()
				s.sendResponse(cc, req.h, invalidRequest, sending)
				sent <- struct{}{}
				return
			}
			s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
			sent <- struct{}{}
		}
	}()

	if timeout == 0 {
		<-called
		<-sent
		return
	}

	select {
	case <-time.After(timeout):
		req.h.Error = errs.ErrServiceHandleTimeout.Error()
		s.sendResponse(cc, req.h, invalidRequest, sending)
		finish <- struct{}{}
	case <-called:
		<-sent
	}
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Panicln("rpc server: read header error: ", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) findService(serviceMethod string) (*service.Service, *service.MethodType, error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		return nil, nil, errs.ErrServiceIllFormed
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		return nil, nil, errs.ErrServiceNotFound
	}
	svc := svci.(*service.Service)
	mtype := svc.Method[methodName]
	if mtype == nil {
		return nil, nil, errs.ErrServiceNotFound
	}
	return svc, mtype, nil
}

func Accept(lis net.Listener, opts ServerOption) {
	DefaultServer.Accept(lis, opts)
}

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
