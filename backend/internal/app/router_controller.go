package app

import (
	"fmt"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/router/infrared"
	"github.com/zurco34/pelican-mc-router/internal/router/mcrouter"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

func newRouteController(
	cfg config.Config,
) (router.RouteController, error) {
	engine, err := router.ParseEngine(cfg.Router.Backend)
	if err != nil {
		return nil, fmt.Errorf(
			"parse router backend: %w",
			err,
		)
	}

	switch engine {
	case router.EngineMCRouter:
		client, err := mcrouter.NewClient(
			mcrouter.ClientConfig{
				BaseURL: cfg.MCRouter.APIURL,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create mc-router client: %w",
				err,
			)
		}

		controller, err := mcrouter.NewController(client)
		if err != nil {
			return nil, fmt.Errorf(
				"create mc-router controller: %w",
				err,
			)
		}

		return controller, nil

	case router.EngineInfrared:
		controller, err := infrared.NewController(
			infrared.Config{
				Directory: cfg.Infrared.ProxiesPath,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create Infrared controller: %w",
				err,
			)
		}

		reloader, err := infrared.NewMarkerReloader(
			cfg.Infrared.ReloadMarkerPath,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create Infrared marker reloader: %w",
				err,
			)
		}

		reloadingController, err :=
			infrared.NewReloadingController(
				controller,
				reloader,
			)
		if err != nil {
			return nil, fmt.Errorf(
				"create reloading Infrared controller: %w",
				err,
			)
		}

		return reloadingController, nil

	default:
		return nil, fmt.Errorf(
			"create route controller: %w: %q",
			router.ErrUnsupportedEngine,
			engine,
		)
	}
}
