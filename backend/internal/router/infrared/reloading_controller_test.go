package infrared

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

type reloadingTestController struct {
	calls   int
	routes  []router.Route
	changed bool
	err     error
}

func (c *reloadingTestController) ReconcileChanges(
	_ context.Context,
	routes []router.Route,
) (bool, error) {
	c.calls++
	c.routes = append([]router.Route(nil), routes...)

	return c.changed, c.err
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

	baseController := &reloadingTestController{
		changed: true,
	}
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

func TestReloadingControllerSkipsReloadWhenConfigurationsUnchanged(
	t *testing.T,
) {
	baseController := &reloadingTestController{
		changed: false,
	}
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

	if baseController.calls != 1 {
		t.Fatalf(
			"base Reconcile() calls = %d, want 1",
			baseController.calls,
		)
	}

	if reloader.calls != 0 {
		t.Fatalf(
			"Reload() calls = %d, want 0",
			reloader.calls,
		)
	}
}

func TestReloadingControllerRetriesPendingReload(t *testing.T) {
	reloadErr := errors.New("reload failed")

	baseController := &reloadingTestController{
		changed: true,
	}
	reloader := &reloadingTestReloader{
		controller: baseController,
		err:        reloadErr,
	}

	controller, err := NewReloadingController(
		baseController,
		reloader,
	)
	if err != nil {
		t.Fatalf("NewReloadingController() error = %v", err)
	}

	err = controller.Reconcile(context.Background(), nil)
	if !errors.Is(err, reloadErr) {
		t.Fatalf(
			"first Reconcile() error = %v, want error %v",
			err,
			reloadErr,
		)
	}

	baseController.changed = false
	reloader.err = nil

	err = controller.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	if baseController.calls != 2 {
		t.Fatalf(
			"base ReconcileChanges() calls = %d, want 2",
			baseController.calls,
		)
	}

	if reloader.calls != 2 {
		t.Fatalf(
			"Reload() calls = %d, want 2",
			reloader.calls,
		)
	}
}

func TestReloadingControllerRetriesReloadAfterCancellation(
	t *testing.T,
) {
	baseController := &reloadingTestController{
		changed: true,
	}
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

	cancelledContext, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	err = controller.Reconcile(cancelledContext, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"first Reconcile() error = %v, want error %v",
			err,
			context.Canceled,
		)
	}

	if reloader.calls != 0 {
		t.Fatalf(
			"Reload() calls after cancellation = %d, want 0",
			reloader.calls,
		)
	}

	baseController.changed = false

	err = controller.Reconcile(context.Background(), nil)
	if err != nil {
		t.Fatalf("second Reconcile() error = %v", err)
	}

	if reloader.calls != 1 {
		t.Fatalf(
			"Reload() calls after retry = %d, want 1",
			reloader.calls,
		)
	}
}
