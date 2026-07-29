package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zurco34/pelican-mc-router/internal/actioncontrol"
	"github.com/zurco34/pelican-mc-router/internal/bootstrap"
	"github.com/zurco34/pelican-mc-router/internal/dashboard"
	"github.com/zurco34/pelican-mc-router/internal/dashboardauth"
	api "github.com/zurco34/pelican-mc-router/internal/http"
	"github.com/zurco34/pelican-mc-router/internal/observability"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/retry"
	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/scheduler"
	"github.com/zurco34/pelican-mc-router/internal/secretfile"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
	"github.com/zurco34/pelican-mc-router/pkg/buildinfo"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := cfg.ValidateInfrastructure(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}

	db, err := sqlite.Open(sqlite.Config{
		Path: cfg.Database.Path,
	})
	if err != nil {
		return fmt.Errorf("open SQLite database: %w", err)
	}
	defer db.Close()

	settingsStore := settings.NewStore(db)
	routePolicyStore := routepolicy.NewStore(db)
	secretReader, err := secretfile.New(cfg.Secrets.Directory)
	if err != nil {
		return fmt.Errorf("create secret reader: %w", err)
	}

	validator := setup.NewPelicanValidator(
		setup.PelicanClientFactoryFunc(
			func(cfg pelican.Config) (setup.PelicanNodeLister, error) {
				return pelican.NewClient(cfg)
			},
		),
		cfg.Pelican.Timeout,
		toRetryConfig(cfg.Retry),
	)

	runtimeManager := runtime.New()
	reconciliationTracker := runtime.NewReconciliationTracker()
	metricsRegistry, reconciliationMetrics, err := observability.NewRegistry()
	if err != nil {
		return fmt.Errorf("create metrics registry: %w", err)
	}

	routeController, err := newRouteController(*cfg)
	if err != nil {
		return fmt.Errorf(
			"create route controller: %w",
			err,
		)
	}

	routeSynchronizer, err := router.NewSynchronizer(
		routeController,
	)
	if err != nil {
		return fmt.Errorf(
			"create route synchronizer: %w",
			err,
		)
	}

	refresher := runtime.NewRefreshTask(
		settingsStore,
		cfg.Pelican.Timeout,
		cfg.Discovery.WildcardBackendHost,
		runtimeManager,
		routeSynchronizer,
		reconciliationTracker,
		reconciliationMetrics,
		toRetryConfig(cfg.Retry),
	).WithSecretResolver(runtime.SecretResolverFunc(secretReader.Read))

	setupService := setup.NewService(
		settingsStore,
		validator,
		refresher,
		runtime.SecretResolverFunc(secretReader.Read),
	)

	if err := refresher.Refresh(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}

		return fmt.Errorf("initialize runtime: %w", err)
	}
	if ctx.Err() != nil {
		return nil
	}

	runtimeScheduler := scheduler.NewTicker()

	apiServer := api.NewServer(
		runtimeManager,
		setupService,
		reconciliationTracker,
		observability.NewHandler(metricsRegistry),
		buildinfo.Current(),
	).WithRoutePolicies(routePolicyStore).WithDashboard(dashboard.NewService(
		setupService,
		reconciliationTracker,
		buildinfo.Current(),
	)).WithActionLimiter(actioncontrol.New(time.Second))
	setupComplete, err := settingsStore.IsSetupComplete()
	if err != nil {
		return fmt.Errorf("determine setup status: %w", err)
	}
	if !setupComplete {
		authorizer, err := bootstrap.New(secretReader, cfg.Secrets.BootstrapTokenName)
		if err != nil {
			return fmt.Errorf("create bootstrap authorizer: %w", err)
		}
		apiServer.WithBootstrapAuthorization(authorizer)
	}
	if cfg.DashboardAuth.Enabled {
		authorizer, err := dashboardauth.New(ctx, cfg.DashboardAuth)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return fmt.Errorf("create dashboard authorizer: %w", err)
		}
		apiServer.WithDashboardAuthorization(authorizer)
		apiServer.WithManagementAuthorization(authorizer)
		apiServer.WithDashboardActions(refresher)
	} else {
		apiServer.WithManagementAuthorization(nil)
	}
	httpRouter := apiServer.Router()

	address := serverAddress(cfg.Server)

	log.Info().
		Str("address", address).
		Str("version", buildinfo.Version).
		Str("revision", buildinfo.Revision).
		Msg("starting HTTP server")

	server := newHTTPServer(address, httpRouter, cfg.Server)

	return runLifecycle(ctx, server, func(runtimeCtx context.Context) error {
		return runtimeScheduler.Run(
			runtimeCtx,
			cfg.Discovery.Interval,
			refresher,
		)
	})
}

func toRetryConfig(cfg config.RetryConfig) retry.Config {
	return retry.Config{
		Attempts:       cfg.Attempts,
		InitialBackoff: cfg.InitialBackoff,
		MaxBackoff:     cfg.MaxBackoff,
	}
}

func newHTTPServer(
	address string,
	handler http.Handler,
	cfg config.ServerConfig,
) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func serverAddress(cfg config.ServerConfig) string {
	return net.JoinHostPort(
		cfg.Host,
		strconv.Itoa(cfg.Port),
	)
}
