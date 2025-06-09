package xclient

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/qiancijun/minirpc/common"
)

type MiniRegisterDiscovery struct {
	*MultiServersDiscovery
	registry   string
	timeout    time.Duration
	lastUpdate time.Time
}

func NewGeeRegistryDiscovery(registerAddr string, timeout time.Duration) *MiniRegisterDiscovery {
	if timeout == 0 {
		timeout = common.DefaultUpdateTimeout
	}
	d := &MiniRegisterDiscovery{
		MultiServersDiscovery: NewMultiServersDiscovery(make([]string, 0)),
		registry:              registerAddr,
		timeout:               timeout,
	}
	return d
}

func (r *MiniRegisterDiscovery) Update(servers []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = servers
	r.lastUpdate = time.Now()
	return nil
}

func (r *MiniRegisterDiscovery) Refresh() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastUpdate.Add(r.timeout).After(time.Now()) {
		return nil
	}
	log.Println("rpc registry: refresh servers from registry", r.registry)
	resp, err := http.Get(r.registry)
	if err != nil {
		log.Println("rpc registry refresh err: ", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-Minirpc-Servers"), ",")
	r.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			r.servers = append(r.servers, strings.TrimSpace(server))
		}
	}
	r.lastUpdate = time.Now()
	return nil
}

func (r *MiniRegisterDiscovery) Get(mode SelectMode) (string, error) {
	if err := r.Refresh(); err != nil {
		return "", err
	}
	return r.MultiServersDiscovery.Get(mode)
}

func (r *MiniRegisterDiscovery) GetAll() ([]string, error) {
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	return r.MultiServersDiscovery.GetAll()
}
