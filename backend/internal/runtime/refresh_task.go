package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/discovery"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/settings"
)

type SettingsStore interface {
	IsSetupComplete() (bool, error)
	Load() (settings.Settings, error)
}

type RouteSynchronizer interface {
	Sync(context.Context, router.RouteSource) error
}

type RefreshTask struct {
	mu sync.Mutex

	store               SettingsStore
	timeout             time.Duration
	wildcardBackendHost string
	manager             *Manager
	synchronizer        RouteSynchronizer
}

func NewRefreshTask(
	store SettingsStore,
	timeout time.Duration,
	wildcardBackendHost string,
	manager *Manager,
	synchronizer RouteSynchronizer,
) *RefreshTask {
	return &RefreshTask{
		store:               store,
		timeout:             timeout,
		wildcardBackendHost: wildcardBackendHost,
		manager:             manager,
		synchronizer:        synchronizer,
	}
}
func (r *RefreshTask) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	discoveryService, routingService, err := r.buildRuntimeServices()
	if err != nil {
		return fmt.Errorf("build runtime services: %w", err)
	}

	if routingService != nil && r.synchronizer != nil {
		if err := r.synchronizer.Sync(ctx, routingService); err != nil {
			return fmt.Errorf(
				"synchronize routes: %w",
				err,
			)
		}
	}

	r.manager.Set(
		discoveryService,
		routingService,
	)

	return nil
}

func (r *RefreshTask) Run(ctx context.Context) error {
	return r.Refresh(ctx)
}

func (r *RefreshTask) buildRuntimeServices() (
	DiscoveryService,
	RoutingService,
	error,
) {
	setupComplete, err := r.store.IsSetupComplete()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"determine setup status: %w",
			err,
		)
	}

	if !setupComplete {
		return nil, nil, nil
	}

	runtimeSettings, err := r.store.Load()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"load runtime settings: %w",
			err,
		)
	}

	pelicanClient, err := pelican.NewClient(pelican.Config{
		BaseURL: runtimeSettings.PelicanURL,
		APIKey:  runtimeSettings.PelicanAPIKey,
		Timeout: r.timeout,
	})
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create Pelican client: %w",
			err,
		)
	}

	discoveryService := discovery.New(
		pelicanClient,
		discovery.WithWildcardBackendHost(
			r.wildcardBackendHost,
		),
	)

	routingService, err := router.New(
		discoveryService,
		runtimeSettings.RouterDomain,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create routing service: %w",
			err,
		)
	}

	return discoveryService, routingService, nil
}
