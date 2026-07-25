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

var (
	ErrRouteClientRequired = errors.New(
		"mcrouter: route client is required",
	)
	ErrDuplicateHostname = errors.New(
		"mcrouter: duplicate route hostname",
	)
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

	desiredBackends := make(
		map[string]string,
		len(routes),
	)
	desiredHostnames := make(
		[]string,
		0,
		len(routes),
	)

	// Validate and prepare the complete desired state before making
	// any calls that could mutate mc-router.
	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"mcrouter: reconcile routes: %w",
				err,
			)
		}

		hostname := route.Hostname

		if _, exists := desiredBackends[hostname]; exists {
			return fmt.Errorf(
				"mcrouter: duplicate route hostname %q: %w",
				hostname,
				ErrDuplicateHostname,
			)
		}

		desiredBackends[hostname] = net.JoinHostPort(
			route.Backend.Host,
			strconv.Itoa(route.Backend.Port),
		)

		desiredHostnames = append(
			desiredHostnames,
			hostname,
		)
	}

	sort.Strings(desiredHostnames)

	currentRoutes, err := c.client.ListRoutes(ctx)
	if err != nil {
		return fmt.Errorf(
			"mcrouter: list current routes: %w",
			err,
		)
	}

	for _, hostname := range desiredHostnames {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"mcrouter: reconcile routes: %w",
				err,
			)
		}

		backend := desiredBackends[hostname]

		if currentRoutes[hostname] == backend {
			continue
		}

		if err := c.client.CreateRoute(
			ctx,
			hostname,
			backend,
		); err != nil {
			return fmt.Errorf(
				"mcrouter: create route %q: %w",
				hostname,
				err,
			)
		}
	}

	staleHostnames := make([]string, 0)

	for hostname := range currentRoutes {
		if _, desired := desiredBackends[hostname]; desired {
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
