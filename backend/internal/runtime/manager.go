package runtime

import (
	"sync"
)

type Manager struct {
	mu sync.RWMutex

	discovery DiscoveryService
	routing   RoutingService
}

func New() *Manager {
	return &Manager{}
}

func (m *Manager) Set(
	discovery DiscoveryService,
	routing RoutingService,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.discovery = discovery
	m.routing = routing
}

func (m *Manager) Discovery() DiscoveryService {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.discovery
}

func (m *Manager) Routing() RoutingService {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.routing
}
