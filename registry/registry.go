package registry

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/qiancijun/minirpc/common"
)

type MiniRegister struct {
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr  string
	start time.Time
}

var (
	DefaultRegistry = NewRegistry(common.DefaultTimeout)
)

func NewRegistry(timeout time.Duration) *MiniRegister {
	return &MiniRegister{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}

func (r *MiniRegister) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{
			Addr:  addr,
			start: time.Now(),
		}
	} else {
		s.start = time.Now()
	}
}

func (r *MiniRegister) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

func (r *MiniRegister) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("X-Minirpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		addr := req.Header.Get("X-Minirpc-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *MiniRegister) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
}

func HandleHTTP() {
	DefaultRegistry.HandleHTTP(common.DefaultPath)
}

func HeartBeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = common.DefaultTimeout - time.Duration(1) * time.Minute
	}
	var err error 
	err = sendHeartBeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<- t.C
			err = sendHeartBeat(registry, addr)
		}
	}()
}

func sendHeartBeat(registry, addr string) error {
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-Minirpc-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		return err
	}
	return nil
}