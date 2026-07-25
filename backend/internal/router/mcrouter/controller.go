package mcrouter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

var ErrRouteClientRequired = errors.New(
	"mcrouter: route client is required",
)

type routeClient interface {
	ListRoutes(context.Context) (map[string]string, error)
	CreateRoute(context.Context, string, string) error
	DeleteRoute(context.Context, string) error
}

type Controller struct {
	mu     sync.Mutex
	client routeClient
}

func NewController(
	client routeClient,
) (*Controller, error) {
	if client == nil {
		return nil, ErrRouteClientRequired
	}

	return &Controller{
		client: client,
	}, nil
}

func (c *Controller) Reconcile(
	ctx context.Context,
	routes []router.Route,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"mcrouter: reconcile routes: %w",
			err,
		)
	}

	currentRoutes, err := c.client.ListRoutes(ctx)
	if err != nil {
		return fmt.Errorf(
			"mcrouter: list current routes: %w",
			err,
		)
	}

	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"mcrouter: reconcile routes: %w",
				err,
			)
		}

		backend := net.JoinHostPort(
			route.Backend.Host,
			strconv.Itoa(route.Backend.Port),
		)

		if currentRoutes[route.Hostname] == backend {
			continue
		}

		if err := c.client.CreateRoute(
			ctx,
			route.Hostname,
			backend,
		); err != nil {
			return fmt.Errorf(
				"mcrouter: create route %q: %w",
				route.Hostname,
				err,
			)
		}
	}

	return nil
}
