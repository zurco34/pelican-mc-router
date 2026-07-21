package runtime

import (
	"context"
	"testing"

	routing "github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type fakeDiscoveryService struct{}

func (*fakeDiscoveryService) Discover(
	context.Context,
) ([]models.MinecraftServer, error) {
	return nil, nil
}

type fakeRoutingService struct{}

func (*fakeRoutingService) Routes(
	context.Context,
) ([]routing.Route, error) {
	return nil, nil
}

func TestNewReturnsEmptyManager(t *testing.T) {
	manager := New()

	if manager == nil {
		t.Fatal("New() returned nil")
	}

	if manager.Discovery() != nil {
		t.Fatal("Discovery() is not nil")
	}

	if manager.Routing() != nil {
		t.Fatal("Routing() is not nil")
	}
}

func TestSetStoresRuntimeServices(t *testing.T) {
	manager := New()

	discovery := &fakeDiscoveryService{}
	routingService := &fakeRoutingService{}

	manager.Set(discovery, routingService)

	if manager.Discovery() != discovery {
		t.Fatal("Discovery() did not return the configured service")
	}

	if manager.Routing() != routingService {
		t.Fatal("Routing() did not return the configured service")
	}
}
func TestManagerSupportsConcurrentAccess(t *testing.T) {
	manager := New()

	discovery := &fakeDiscoveryService{}
	routingService := &fakeRoutingService{}

	const iterations = 1000

	done := make(chan struct{})

	go func() {
		defer close(done)

		for range iterations {
			manager.Set(discovery, routingService)
		}
	}()

	for range iterations {
		_ = manager.Discovery()
		_ = manager.Routing()
	}

	<-done
}
