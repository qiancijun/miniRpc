package xclient

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/qiancijun/minirpc/errs"
)

type MultiServersDiscovery struct {
	r       *rand.Rand
	mu      sync.RWMutex
	servers []string
	index   int
}

// Get implements Discovery.
func (m *MultiServersDiscovery) Get(mode SelectMode) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.servers)
	if n == 0 {
		return "", errs.ErrNoAvailableServers
	}
	switch mode {
	case RandomSelect:
		return m.servers[m.r.Intn(n)], nil
	case RoundRobinSelect:
		s := m.servers[m.index%n]
		m.index = (m.index + 1) % n
		return s, nil
	default:
		return "", errs.ErrNotSupportedSelectMode
	}
}

// GetAll implements Discovery.
func (m *MultiServersDiscovery) GetAll() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	servers := make([]string, len(m.servers))
	copy(servers, m.servers)
	return servers, nil
}

// Refresh implements Discovery.
func (m *MultiServersDiscovery) Refresh() error {
	return nil
}

// Update implements Discovery.
func (m *MultiServersDiscovery) Update(servers []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = servers
	return nil
}

func NewMultiServersDiscovery(servers []string) *MultiServersDiscovery {
	d := &MultiServersDiscovery{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	d.index = d.r.Intn(math.MaxInt32 - 1)
	return d
}

var _ Discovery = (*MultiServersDiscovery)(nil)
