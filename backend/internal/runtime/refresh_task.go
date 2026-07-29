package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
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

type SecretResolver interface {
	Resolve(string) ([]byte, error)
}

type SecretResolverFunc func(string) ([]byte, error)

func (f SecretResolverFunc) Resolve(name string) ([]byte, error) { return f(name) }

type RouteSynchronizer interface {
	Sync(context.Context, router.RouteSource) error
}

type DiagnosticsRouteSynchronizer interface {
	SyncWithResult(context.Context, router.RouteSource) (router.ReconciliationResult, error)
}

type ReconciliationObserver interface {
	ObserveReconciliation(ReconciliationStatus)
}

// ReconciliationEventRecorder persists an allowlisted reconciliation outcome.
// Implementations must not retain errors, route identities, or other topology.
type ReconciliationEventRecorder interface {
	RecordReconciliation(context.Context, ReconciliationOutcome, router.ReconciliationResult) error
}

type RefreshTask struct {
	refreshLock chan struct{}

	store               SettingsStore
	timeout             time.Duration
	retry               retry.Config
	wildcardBackendHost string
	manager             *Manager
	synchronizer        RouteSynchronizer
	tracker             *ReconciliationTracker
	observer            ReconciliationObserver
	recorder            ReconciliationEventRecorder
	secretResolver      SecretResolver
	policySource        router.PolicySource
}

func (r *RefreshTask) WithPolicySource(source router.PolicySource) *RefreshTask {
	r.policySource = source
	return r
}

func (r *RefreshTask) WithSecretResolver(resolver SecretResolver) *RefreshTask {
	r.secretResolver = resolver
	return r
}

func (r *RefreshTask) WithReconciliationEventRecorder(recorder ReconciliationEventRecorder) *RefreshTask {
	r.recorder = recorder
	return r
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
		refreshLock:         make(chan struct{}, 1),
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
	if err := r.acquire(ctx); err != nil {
		return err
	}
	defer r.release()
	r.tracker.Start()
	r.observe()

	discoveryService, routingService, err := r.buildRuntimeServices()
	if err != nil {
		r.tracker.CompleteRuntimeBuildFailure()
		r.observe()
		r.record(ctx)
		return fmt.Errorf("build runtime services: %w", err)
	}
	if routingService == nil {
		r.manager.Set(nil, nil)
		r.tracker.CompleteNotConfigured()
		r.observe()
		r.record(ctx)
		return nil
	}
	if _, productionSynchronizer := r.synchronizer.(*router.Synchronizer); productionSynchronizer {
		inventory, ok := discoveryService.(*Inventory)
		if !ok {
			return fmt.Errorf("refresh inventory: inventory service is unavailable")
		}
		if err := inventory.Refresh(ctx); err != nil {
			r.tracker.CompleteRuntimeBuildFailure()
			r.observe()
			r.record(ctx)
			return fmt.Errorf("refresh inventory: %w", err)
		}
	}

	result := router.ReconciliationResult{}
	if r.synchronizer != nil {
		result, err = synchronizeRoutes(ctx, r.synchronizer, routingService)
		if err != nil {
			r.tracker.CompleteRouteSynchronizationFailure(result)
			r.observe()
			r.record(ctx)
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
	r.tracker.CompleteSuccess(result)
	r.observe()
	r.record(ctx)
	log.Info().
		Int("desired_routes", result.Desired).
		Int("created_routes", result.Created).
		Int("updated_routes", result.Updated).
		Int("deleted_routes", result.Deleted).
		Bool("routes_changed", result.Changed).
		Msg("reconciliation completed")

	return nil
}

func (r *RefreshTask) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case r.refreshLock <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *RefreshTask) release() {
	<-r.refreshLock
}

func synchronizeRoutes(
	ctx context.Context,
	synchronizer RouteSynchronizer,
	source router.RouteSource,
) (router.ReconciliationResult, error) {
	if diagnostics, ok := synchronizer.(DiagnosticsRouteSynchronizer); ok {
		return diagnostics.SyncWithResult(ctx, source)
	}

	err := synchronizer.Sync(ctx, source)
	return router.ReconciliationResult{}, err
}

func (r *RefreshTask) observe() {
	if r.observer != nil {
		r.observer.ObserveReconciliation(r.tracker.Snapshot())
	}
}

func (r *RefreshTask) record(ctx context.Context) {
	if r.recorder == nil {
		return
	}

	status := r.tracker.Snapshot()
	if status.LastOutcome == nil || status.InProgress {
		return
	}
	if err := r.recorder.RecordReconciliation(ctx, *status.LastOutcome, status.RouteChanges); err != nil {
		log.Warn().Msg("record reconciliation history failed")
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
	if runtimeSettings.PelicanSecretName != "" {
		if r.secretResolver == nil {
			return nil, nil, fmt.Errorf("resolve Pelican credential: secret resolver is unavailable")
		}
		secret, err := r.secretResolver.Resolve(runtimeSettings.PelicanSecretName)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve Pelican credential: %w", err)
		}
		runtimeSettings.PelicanAPIKey = string(secret)
		clear(secret)
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

	inventory := NewInventory(discoveryService)
	routingService, err := router.New(
		inventory,
		runtimeSettings.RouterDomain,
		router.WithPolicySource(r.policySource),
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create routing service: %w",
			err,
		)
	}

	return inventory, routingService, nil
}
