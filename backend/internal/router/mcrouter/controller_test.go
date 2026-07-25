package mcrouter

import (
	"context"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

type fakeRouteClient struct {
	routes  map[string]string
	created map[string]string
	deleted []string
}

func (c *fakeRouteClient) ListRoutes(
	context.Context,
) (map[string]string, error) {
	routes := make(map[string]string, len(c.routes))

	for hostname, backend := range c.routes {
		routes[hostname] = backend
	}

	return routes, nil
}

func (c *fakeRouteClient) CreateRoute(
	_ context.Context,
	hostname string,
	backend string,
) error {
	if c.created == nil {
		c.created = make(map[string]string)
	}

	c.created[hostname] = backend

	return nil
}

func (c *fakeRouteClient) DeleteRoute(
	_ context.Context,
	hostname string,
) error {
	c.deleted = append(c.deleted, hostname)

	return nil
}

func TestControllerReconcileCreatesMissingRoute(t *testing.T) {
	t.Parallel()

	client := &fakeRouteClient{
		routes: make(map[string]string),
	}

	controller, err := NewController(client)
	if err != nil {
		t.Fatalf("NewController() error = %v", err)
	}

	err = controller.Reconcile(
		context.Background(),
		[]router.Route{
			{
				ServerID: "server-123",
				Hostname: "survival.mc.example.com",
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 25565,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	const hostname = "survival.mc.example.com"
	const backend = "10.0.0.25:25565"

	if got := client.created[hostname]; got != backend {
		t.Fatalf(
			"created route backend = %q, want %q",
			got,
			backend,
		)
	}

	if len(client.deleted) != 0 {
		t.Fatalf(
			"deleted routes = %#v, want none",
			client.deleted,
		)
	}
}
