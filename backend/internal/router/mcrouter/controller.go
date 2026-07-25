package mcrouter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

var (
	ErrRouteClientRequired = errors.New(
		"mcrouter: route client is required",
	)
	ErrEmptyHostname = errors.New(
		"mcrouter: route hostname must not be empty",
	)
	ErrInvalidHostname = errors.New(
		"mcrouter: route hostname is not a valid DNS name",
	)
	ErrEmptyBackendHost = errors.New(
		"mcrouter: backend host must not be empty",
	)
	ErrInvalidBackendPort = errors.New(
		"mcrouter: backend port must be between 1 and 65535",
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

func isValidHostname(hostname string) bool {
	if len(hostname) > 253 ||
		strings.HasSuffix(hostname, ".") {
		return false
	}

	labels := strings.Split(hostname, ".")

	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}

		if label[0] == '-' ||
			label[len(label)-1] == '-' {
			return false
		}

		for index := 0; index < len(label); index++ {
			character := label[index]

			if character >= 'a' && character <= 'z' {
				continue
			}

			if character >= '0' && character <= '9' {
				continue
			}

			if character == '-' {
				continue
			}

			return false
		}
	}

	return true
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
	// any mc-router API calls.
	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(
				"mcrouter: reconcile routes: %w",
				err,
			)
		}

		hostname := strings.ToLower(
			strings.TrimSpace(route.Hostname),
		)

		if hostname == "" {
			return fmt.Errorf(
				"mcrouter: validate route for server %q: %w",
				route.ServerID,
				ErrEmptyHostname,
			)
		}

		if !isValidHostname(hostname) {
			return fmt.Errorf(
				"mcrouter: validate route for server %q: "+
					"hostname %q: %w",
				route.ServerID,
				hostname,
				ErrInvalidHostname,
			)
		}

		backendHost := strings.TrimSpace(route.Backend.Host)

		if backendHost == "" {
			return fmt.Errorf(
				"mcrouter: validate route for server %q: %w",
				route.ServerID,
				ErrEmptyBackendHost,
			)
		}

		if route.Backend.Port < 1 ||
			route.Backend.Port > 65535 {
			return fmt.Errorf(
				"mcrouter: validate route for server %q: "+
					"backend port %d: %w",
				route.ServerID,
				route.Backend.Port,
				ErrInvalidBackendPort,
			)
		}

		if _, exists := desiredBackends[hostname]; exists {
			return fmt.Errorf(
				"mcrouter: duplicate route hostname %q: %w",
				hostname,
				ErrDuplicateHostname,
			)
		}

		desiredBackends[hostname] = net.JoinHostPort(
			backendHost,
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
