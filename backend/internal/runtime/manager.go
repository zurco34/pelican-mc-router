package runtime

import (
	"sync"

	api "github.com/zurco34/pelican-mc-router/internal/http"
)

type Manager struct {
	mu sync.RWMutex

	discovery api.DiscoveryService
	routing   api.RoutingService
}

func New() *Manager {
	return &Manager{}
}

func (m *Manager) Set(
	discovery api.DiscoveryService,
	routing api.RoutingService,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.discovery = discovery
	m.routing = routing
}

func (m *Manager) Discovery() api.DiscoveryService {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.discovery
}

func (m *Manager) Routing() api.RoutingService {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.routing
}
