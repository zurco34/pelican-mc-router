package runtime

import (
	"context"
	"errors"
	"sync"

	"github.com/zurco34/pelican-mc-router/pkg/models"
)

var ErrInventoryUnavailable = errors.New("inventory snapshot unavailable")

// Inventory caches the last successful discovery result for read-only runtime
// consumers. Refresh is the only operation that performs upstream I/O.
type Inventory struct {
	mu      sync.RWMutex
	source  DiscoveryService
	servers []models.MinecraftServer
	ready   bool
}

func NewInventory(source DiscoveryService) *Inventory { return &Inventory{source: source} }

func (i *Inventory) Refresh(ctx context.Context) error {
	servers, err := i.source.Discover(ctx)
	if err != nil {
		return err
	}
	i.mu.Lock()
	i.servers = append(i.servers[:0], servers...)
	i.ready = true
	i.mu.Unlock()
	return nil
}

func (i *Inventory) Discover(context.Context) ([]models.MinecraftServer, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if !i.ready {
		return nil, ErrInventoryUnavailable
	}
	return append([]models.MinecraftServer(nil), i.servers...), nil
}
