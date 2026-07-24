package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	api "github.com/zurco34/pelican-mc-router/internal/http"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/router/infrared"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/scheduler"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

func Run() error {
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

	infraredController, err := infrared.NewController(
		infrared.Config{
			Directory: cfg.Infrared.ProxiesPath,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"create Infrared controller: %w",
			err,
		)
	}

	infraredReloader, err := infrared.NewMarkerReloader(
		cfg.Infrared.ReloadMarkerPath,
	)
	if err != nil {
		return fmt.Errorf(
			"create Infrared marker reloader: %w",
			err,
		)
	}

	reloadingInfraredController, err := infrared.NewReloadingController(
		infraredController,
		infraredReloader,
	)
	if err != nil {
		return fmt.Errorf(
			"create reloading Infrared controller: %w",
			err,
		)
	}

	routeSynchronizer, err := router.NewSynchronizer(
		reloadingInfraredController,
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
		runtimeManager,
		routeSynchronizer,
	)

	setupService := setup.NewService(
		settingsStore,
		validator,
		refresher,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := refresher.Refresh(ctx); err != nil {
		return fmt.Errorf("initialize runtime: %w", err)
	}

	runtimeScheduler := scheduler.NewTicker()

	schedulerErrors := make(chan error, 1)

	go func() {
		schedulerErrors <- runtimeScheduler.Run(
			ctx,
			cfg.Discovery.Interval,
			refresher,
		)
	}()

	httpRouter := api.NewServer(
		runtimeManager,
		setupService,
	).Router()

	address := serverAddress(cfg.Server)

	log.Info().
		Str("address", address).
		Msg("starting HTTP server")

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- http.ListenAndServe(address, httpRouter)
	}()

	select {
	case err := <-serverErrors:
		cancel()

		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}

		return nil

	case err := <-schedulerErrors:
		cancel()

		if err != nil {
			return fmt.Errorf("run runtime scheduler: %w", err)
		}

		return nil
	}
}

func serverAddress(cfg config.ServerConfig) string {
	return net.JoinHostPort(
		cfg.Host,
		strconv.Itoa(cfg.Port),
	)
}
