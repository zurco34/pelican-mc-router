package router

import (
	"context"
	"reflect"
	"testing"
)

type synchronizerRouteSource struct {
	routes []Route
	err    error
}

func (s *synchronizerRouteSource) Routes(
	context.Context,
) ([]Route, error) {
	return s.routes, s.err
}

type synchronizerRouteController struct {
	routes []Route
	calls  int
	err    error
}

func (c *synchronizerRouteController) Reconcile(
	_ context.Context,
	routes []Route,
) error {
	c.calls++
	c.routes = append([]Route(nil), routes...)

	return c.err
}

func TestSynchronizerAppliesGeneratedRoutes(t *testing.T) {
	expected := []Route{
		{
			ServerID: "server-123",
			Hostname: "survival.mc.example.com",
			Backend: Backend{
				Host: "10.0.0.25",
				Port: 25565,
			},
		},
	}

	source := &synchronizerRouteSource{
		routes: expected,
	}
	controller := &synchronizerRouteController{}

	synchronizer, err := NewSynchronizer(controller)
	if err != nil {
		t.Fatalf("NewSynchronizer() error = %v", err)
	}

	err = synchronizer.Sync(context.Background(), source)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if controller.calls != 1 {
		t.Fatalf(
			"Reconcile() calls = %d, want 1",
			controller.calls,
		)
	}

	if !reflect.DeepEqual(controller.routes, expected) {
		t.Fatalf(
			"Reconcile() routes = %#v, want %#v",
			controller.routes,
			expected,
		)
	}
}
