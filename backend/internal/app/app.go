package app

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"
	"github.com/zurco34/pelican-mc-router/internal/discovery"
	api "github.com/zurco34/pelican-mc-router/internal/http"
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate configuration: %w", err)
	}
	pelicanClient, err := pelican.NewClient(pelican.Config{
		BaseURL: cfg.Pelican.URL,
		APIKey:  cfg.Pelican.APIKey,
		Timeout: cfg.Pelican.Timeout,
	})
	if err != nil {
		return fmt.Errorf("create Pelican client: %w", err)
	}

	discoveryService := discovery.New(pelicanClient)

	routingService, err := router.New(
		discoveryService,
		cfg.Router.Domain,
	)
	if err != nil {
		return fmt.Errorf("create routing service: %w", err)
	}
	runtimeManager := runtime.New()
	runtimeManager.Set(
		discoveryService,
		routingService,
	)
	httpRouter := api.NewServer(
		runtimeManager,
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

func serverAddress(cfg config.ServerConfig) string {
	return net.JoinHostPort(
		cfg.Host,
		strconv.Itoa(cfg.Port),
	)
}
