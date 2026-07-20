package app

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"

	api "github.com/zurco34/pelican-mc-router/internal/http"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	address := serverAddress(cfg.Server)

	router := api.NewRouter()

	log.Info().
		Str("address", address).
		Msg("starting HTTP server")

	if err := http.ListenAndServe(address, router); err != nil {
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
