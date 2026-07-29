package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	api "github.com/zurco34/pelican-mc-router/internal/http"
	"github.com/zurco34/pelican-mc-router/internal/observability"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/scheduler"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
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

	validator := setup.NewPelicanValidator(
		setup.PelicanClientFactoryFunc(
			func(cfg pelican.Config) (setup.PelicanNodeLister, error) {
				return pelican.NewClient(cfg)
			},
		),
		cfg.Pelican.Timeout,
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
	)

	setupService := setup.NewService(
		settingsStore,
		validator,
		refresher,
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

	httpRouter := api.NewServer(
		runtimeManager,
		setupService,
		reconciliationTracker,
		observability.NewHandler(metricsRegistry),
	).Router()

	address := serverAddress(cfg.Server)

	log.Info().
		Str("address", address).
		Msg("starting HTTP server")

	server := &http.Server{
		Addr:    address,
		Handler: httpRouter,
	}

	return runLifecycle(ctx, server, func(runtimeCtx context.Context) error {
		return runtimeScheduler.Run(
			runtimeCtx,
			cfg.Discovery.Interval,
			refresher,
		)
	})
}

func serverAddress(cfg config.ServerConfig) string {
	return net.JoinHostPort(
		cfg.Host,
		strconv.Itoa(cfg.Port),
	)
}
