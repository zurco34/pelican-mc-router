package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/discovery"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/retry"
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

type ReconciliationObserver interface {
	ObserveReconciliation(ReconciliationStatus)
}

type RefreshTask struct {
	mu sync.Mutex

	store               SettingsStore
	timeout             time.Duration
	retry               retry.Config
	wildcardBackendHost string
	manager             *Manager
	synchronizer        RouteSynchronizer
	tracker             *ReconciliationTracker
	observer            ReconciliationObserver
}

func NewRefreshTask(
	store SettingsStore,
	timeout time.Duration,
	wildcardBackendHost string,
	manager *Manager,
	synchronizer RouteSynchronizer,
	tracker *ReconciliationTracker,
	observer ReconciliationObserver,
	retryConfigs ...retry.Config,
) *RefreshTask {
	var retryConfig retry.Config
	if len(retryConfigs) > 0 {
		retryConfig = retryConfigs[0]
	}

	return &RefreshTask{
		store:               store,
		timeout:             timeout,
		retry:               retryConfig,
		wildcardBackendHost: wildcardBackendHost,
		manager:             manager,
		synchronizer:        synchronizer,
		tracker:             tracker,
		observer:            observer,
	}
}
func (r *RefreshTask) Refresh(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tracker.Start()
	r.observe()

	discoveryService, routingService, err := r.buildRuntimeServices()
	if err != nil {
		r.tracker.CompleteRuntimeBuildFailure()
		r.observe()
		return fmt.Errorf("build runtime services: %w", err)
	}
	if routingService == nil {
		r.manager.Set(nil, nil)
		r.tracker.CompleteNotConfigured()
		r.observe()
		return nil
	}

	if r.synchronizer != nil {
		if err := r.synchronizer.Sync(ctx, routingService); err != nil {
			r.tracker.CompleteRouteSynchronizationFailure()
			r.observe()
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
	r.tracker.CompleteSuccess()
	r.observe()

	return nil
}

func (r *RefreshTask) observe() {
	if r.observer != nil {
		r.observer.ObserveReconciliation(r.tracker.Snapshot())
	}
}

func (r *RefreshTask) ReconciliationTracker() *ReconciliationTracker {
	return r.tracker
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
		Retry:   r.retry,
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
