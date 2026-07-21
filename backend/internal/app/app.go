package app

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zurco34/pelican-mc-router/internal/discovery"
	api "github.com/zurco34/pelican-mc-router/internal/http"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

type runtimeSettingsStore interface {
	IsSetupComplete() (bool, error)
	Load() (settings.Settings, error)
}

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

	setupService := setup.NewService(
		settingsStore,
		validator,
	)

	discoveryService, routingService, err := buildRuntimeServices(
		settingsStore,
		cfg.Pelican.Timeout,
	)
	if err != nil {
		return err
	}
	runtimeManager := runtime.New()
	runtimeManager.Set(
		discoveryService,
		routingService,
	)

	httpRouter := api.NewServer(
		runtimeManager,
		setupService,
	).Router()

	address := serverAddress(cfg.Server)

	log.Info().
		Str("address", address).
		Msg("starting HTTP server")

	if err := http.ListenAndServe(address, httpRouter); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}
func buildRuntimeServices(
	store runtimeSettingsStore,
	timeout time.Duration,
) (runtime.DiscoveryService, runtime.RoutingService, error) {
	setupComplete, err := store.IsSetupComplete()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"determine setup status: %w",
			err,
		)
	}

	if !setupComplete {
		return nil, nil, nil
	}

	runtimeSettings, err := store.Load()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"load runtime settings: %w",
			err,
		)
	}

	pelicanClient, err := pelican.NewClient(pelican.Config{
		BaseURL: runtimeSettings.PelicanURL,
		APIKey:  runtimeSettings.PelicanAPIKey,
		Timeout: timeout,
	})
	if err != nil {
		return nil, nil, fmt.Errorf(
			"create Pelican client: %w",
			err,
		)
	}

	discoveryService := discovery.New(pelicanClient)

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

func serverAddress(cfg config.ServerConfig) string {
	return net.JoinHostPort(
		cfg.Host,
		strconv.Itoa(cfg.Port),
	)
}
