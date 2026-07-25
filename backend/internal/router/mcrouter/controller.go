package mcrouter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
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

	desiredHostnames := make(
		map[string]struct{},
		len(routes),
	)

	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"mcrouter: reconcile routes: %w",
				err,
			)
		}

		desiredHostnames[route.Hostname] = struct{}{}

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

	staleHostnames := make([]string, 0)

	for hostname := range currentRoutes {
		if _, desired := desiredHostnames[hostname]; desired {
			continue
		}

		staleHostnames = append(
			staleHostnames,
			hostname,
		)
	}

	sort.Strings(staleHostnames)

	for _, hostname := range staleHostnames {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"mcrouter: reconcile routes: %w",
				err,
			)
		}

		if err := c.client.DeleteRoute(
			ctx,
			hostname,
		); err != nil {
			return fmt.Errorf(
				"mcrouter: delete stale route %q: %w",
				hostname,
				err,
			)
		}
	}

	return nil
}
