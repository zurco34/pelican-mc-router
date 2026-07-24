package infrared

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

var (
	errRouteControllerRequired = errors.New(
		"infrared: route controller is required",
	)
	errReloaderRequired = errors.New(
		"infrared: reloader is required",
	)
)

type ChangeAwareRouteController interface {
	ReconcileChanges(
		context.Context,
		[]router.Route,
	) (bool, error)
}

type Reloader interface {
	Reload(context.Context) error
}

type ReloadingController struct {
	mu            sync.Mutex
	controller    ChangeAwareRouteController
	reloader      Reloader
	reloadPending bool
}

func NewReloadingController(
	controller ChangeAwareRouteController,
	reloader Reloader,
) (*ReloadingController, error) {
	if controller == nil {
		return nil, errRouteControllerRequired
	}

	if reloader == nil {
		return nil, errReloaderRequired
	}

	return &ReloadingController{
		controller: controller,
		reloader:   reloader,
	}, nil
}

func (c *ReloadingController) Reconcile(
	ctx context.Context,
	routes []router.Route,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	changed, err := c.controller.ReconcileChanges(ctx, routes)
	if err != nil {
		return fmt.Errorf(
			"infrared: reconcile proxy configurations: %w",
			err,
		)
	}

	if changed {
		c.reloadPending = true
	}

	if !c.reloadPending {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"infrared: reload proxy configurations: %w",
			err,
		)
	}

	if err := c.reloader.Reload(ctx); err != nil {
		return fmt.Errorf(
			"infrared: reload proxy configurations: %w",
			err,
		)
	}

	c.reloadPending = false

	return nil
}
