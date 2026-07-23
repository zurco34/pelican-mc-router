package infrared

import (
	"context"
	"reflect"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

type reloadingTestController struct {
	calls  int
	routes []router.Route
	err    error
}

func (c *reloadingTestController) Reconcile(
	_ context.Context,
	routes []router.Route,
) error {
	c.calls++
	c.routes = append([]router.Route(nil), routes...)

	return c.err
}

type reloadingTestReloader struct {
	controller              *reloadingTestController
	calls                   int
	controllerCallsAtReload int
	err                     error
}

func (r *reloadingTestReloader) Reload(context.Context) error {
	r.calls++
	r.controllerCallsAtReload = r.controller.calls

	return r.err
}

func TestReloadingControllerReloadsAfterReconciliation(t *testing.T) {
	expected := []router.Route{
		{
			ServerID: "server-123",
			Hostname: "survival.mc.example.com",
			Backend: router.Backend{
				Host: "10.0.0.25",
				Port: 25565,
			},
		},
	}

	baseController := &reloadingTestController{}
	reloader := &reloadingTestReloader{
		controller: baseController,
	}

	controller, err := NewReloadingController(
		baseController,
		reloader,
	)
	if err != nil {
		t.Fatalf("NewReloadingController() error = %v", err)
	}

	err = controller.Reconcile(
		context.Background(),
		expected,
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if baseController.calls != 1 {
		t.Fatalf(
			"base Reconcile() calls = %d, want 1",
			baseController.calls,
		)
	}

	if reloader.calls != 1 {
		t.Fatalf(
			"Reload() calls = %d, want 1",
			reloader.calls,
		)
	}

	if reloader.controllerCallsAtReload != 1 {
		t.Fatalf(
			"base Reconcile() calls when Reload() ran = %d, want 1",
			reloader.controllerCallsAtReload,
		)
	}

	if !reflect.DeepEqual(baseController.routes, expected) {
		t.Fatalf(
			"base Reconcile() routes = %#v, want %#v",
			baseController.routes,
			expected,
		)
	}
}
