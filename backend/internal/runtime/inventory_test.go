package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type inventorySource struct {
	servers []models.MinecraftServer
	err     error
	calls   int
}

func (s *inventorySource) Discover(context.Context) ([]models.MinecraftServer, error) {
	s.calls++
	return s.servers, s.err
}

func TestInventoryServesOnlyLastSuccessfulSnapshot(t *testing.T) {
	t.Parallel()
	source := &inventorySource{servers: []models.MinecraftServer{{UUID: "server"}}}
	inventory := NewInventory(source)
	if _, err := inventory.Discover(context.Background()); !errors.Is(err, ErrInventoryUnavailable) {
		t.Fatalf("Discover() error = %v", err)
	}
	if err := inventory.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	servers, err := inventory.Discover(context.Background())
	if err != nil || len(servers) != 1 || source.calls != 1 {
		t.Fatalf("snapshot = %#v, error = %v, calls = %d", servers, err, source.calls)
	}
	servers[0].UUID = "mutated"
	again, err := inventory.Discover(context.Background())
	if err != nil || again[0].UUID != "server" {
		t.Fatalf("snapshot was mutable: %#v, error = %v", again, err)
	}
}
