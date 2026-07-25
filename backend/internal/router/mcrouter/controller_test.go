package mcrouter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

type fakeRouteClient struct {
	listCalls int
	routes    map[string]string
	created   map[string]string
	deleted   []string
}

func (c *fakeRouteClient) ListRoutes(
	context.Context,
) (map[string]string, error) {
	c.listCalls++

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

func TestControllerReconcileUpdatesChangedRoute(t *testing.T) {
	t.Parallel()

	const hostname = "survival.mc.example.com"

	client := &fakeRouteClient{
		routes: map[string]string{
			hostname: "10.0.0.24:25565",
		},
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
				Hostname: hostname,
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 25566,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	const expectedBackend = "10.0.0.25:25566"

	if got := client.created[hostname]; got != expectedBackend {
		t.Fatalf(
			"updated route backend = %q, want %q",
			got,
			expectedBackend,
		)
	}

	if len(client.deleted) != 0 {
		t.Fatalf(
			"deleted routes = %#v, want none",
			client.deleted,
		)
	}
}

func TestControllerReconcileDeletesStaleRoute(t *testing.T) {
	t.Parallel()

	const (
		desiredHostname = "survival.mc.example.com"
		staleHostname   = "creative.mc.example.com"
		backend         = "10.0.0.25:25565"
	)

	client := &fakeRouteClient{
		routes: map[string]string{
			desiredHostname: backend,
			staleHostname:   "10.0.0.26:25566",
		},
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
				Hostname: desiredHostname,
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

	if len(client.created) != 0 {
		t.Fatalf(
			"created routes = %#v, want none",
			client.created,
		)
	}

	if len(client.deleted) != 1 {
		t.Fatalf(
			"deleted routes = %#v, want [%q]",
			client.deleted,
			staleHostname,
		)
	}

	if client.deleted[0] != staleHostname {
		t.Fatalf(
			"deleted route = %q, want %q",
			client.deleted[0],
			staleHostname,
		)
	}
}

func TestControllerReconcileRejectsDuplicateHostnameBeforeMutations(
	t *testing.T,
) {
	t.Parallel()

	const hostname = "survival.mc.example.com"

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
				Hostname: hostname,
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 25565,
				},
			},
			{
				ServerID: "server-456",
				Hostname: hostname,
				Backend: router.Backend{
					Host: "10.0.0.26",
					Port: 25566,
				},
			},
		},
	)

	if !errors.Is(err, ErrDuplicateHostname) {
		t.Fatalf(
			"Reconcile() error = %v, want %v",
			err,
			ErrDuplicateHostname,
		)
	}

	if len(client.created) != 0 {
		t.Fatalf(
			"created routes = %#v, want none",
			client.created,
		)
	}

	if len(client.deleted) != 0 {
		t.Fatalf(
			"deleted routes = %#v, want none",
			client.deleted,
		)
	}
}

func TestControllerReconcileRejectsEmptyHostnameBeforeAPICalls(
	t *testing.T,
) {
	t.Parallel()

	client := &fakeRouteClient{
		routes: map[string]string{
			"existing.mc.example.com": "10.0.0.24:25565",
		},
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
				Hostname: "   ",
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 25565,
				},
			},
		},
	)

	if !errors.Is(err, ErrEmptyHostname) {
		t.Fatalf(
			"Reconcile() error = %v, want %v",
			err,
			ErrEmptyHostname,
		)
	}

	if client.listCalls != 0 {
		t.Fatalf(
			"ListRoutes() calls = %d, want 0",
			client.listCalls,
		)
	}

	if len(client.created) != 0 {
		t.Fatalf(
			"created routes = %#v, want none",
			client.created,
		)
	}

	if len(client.deleted) != 0 {
		t.Fatalf(
			"deleted routes = %#v, want none",
			client.deleted,
		)
	}
}

func TestControllerReconcileRejectsEmptyBackendHostBeforeAPICalls(
	t *testing.T,
) {
	t.Parallel()

	client := &fakeRouteClient{
		routes: map[string]string{
			"existing.mc.example.com": "10.0.0.24:25565",
		},
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
					Host: "   ",
					Port: 25565,
				},
			},
		},
	)

	if !errors.Is(err, ErrEmptyBackendHost) {
		t.Fatalf(
			"Reconcile() error = %v, want %v",
			err,
			ErrEmptyBackendHost,
		)
	}

	if client.listCalls != 0 {
		t.Fatalf(
			"ListRoutes() calls = %d, want 0",
			client.listCalls,
		)
	}

	if len(client.created) != 0 {
		t.Fatalf(
			"created routes = %#v, want none",
			client.created,
		)
	}

	if len(client.deleted) != 0 {
		t.Fatalf(
			"deleted routes = %#v, want none",
			client.deleted,
		)
	}
}

func TestControllerReconcileRejectsInvalidBackendPortBeforeAPICalls(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name string
		port int
	}{
		{
			name: "zero",
			port: 0,
		},
		{
			name: "negative",
			port: -1,
		},
		{
			name: "above maximum",
			port: 65536,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := &fakeRouteClient{
				routes: map[string]string{
					"existing.mc.example.com": "10.0.0.24:25565",
				},
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
							Port: test.port,
						},
					},
				},
			)

			if !errors.Is(err, ErrInvalidBackendPort) {
				t.Fatalf(
					"Reconcile() error = %v, want %v",
					err,
					ErrInvalidBackendPort,
				)
			}

			if client.listCalls != 0 {
				t.Fatalf(
					"ListRoutes() calls = %d, want 0",
					client.listCalls,
				)
			}

			if len(client.created) != 0 {
				t.Fatalf(
					"created routes = %#v, want none",
					client.created,
				)
			}

			if len(client.deleted) != 0 {
				t.Fatalf(
					"deleted routes = %#v, want none",
					client.deleted,
				)
			}
		})
	}
}

func TestControllerReconcileRejectsInvalidHostnameBeforeAPICalls(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name     string
		hostname string
	}{
		{
			name:     "empty label",
			hostname: "survival..mc.example.com",
		},
		{
			name:     "label starts with hyphen",
			hostname: "-survival.mc.example.com",
		},
		{
			name:     "label ends with hyphen",
			hostname: "survival-.mc.example.com",
		},
		{
			name:     "invalid character",
			hostname: "survival_mc.example.com",
		},
		{
			name:     "wildcard",
			hostname: "*.mc.example.com",
		},
		{
			name: "label exceeds 63 characters",
			hostname: strings.Repeat("a", 64) +
				".mc.example.com",
		},
		{
			name: "hostname exceeds 253 characters",
			hostname: strings.Repeat("a.", 126) +
				"com",
		},
		{
			name:     "trailing dot",
			hostname: "survival.mc.example.com.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := &fakeRouteClient{
				routes: map[string]string{
					"existing.mc.example.com": "10.0.0.24:25565",
				},
			}

			controller, err := NewController(client)
			if err != nil {
				t.Fatalf(
					"NewController() error = %v",
					err,
				)
			}

			err = controller.Reconcile(
				context.Background(),
				[]router.Route{
					{
						ServerID: "server-123",
						Hostname: test.hostname,
						Backend: router.Backend{
							Host: "10.0.0.25",
							Port: 25565,
						},
					},
				},
			)

			if !errors.Is(err, ErrInvalidHostname) {
				t.Fatalf(
					"Reconcile() error = %v, want %v",
					err,
					ErrInvalidHostname,
				)
			}

			if client.listCalls != 0 {
				t.Fatalf(
					"ListRoutes() calls = %d, want 0",
					client.listCalls,
				)
			}

			if len(client.created) != 0 {
				t.Fatalf(
					"created routes = %#v, want none",
					client.created,
				)
			}

			if len(client.deleted) != 0 {
				t.Fatalf(
					"deleted routes = %#v, want none",
					client.deleted,
				)
			}
		})
	}
}
