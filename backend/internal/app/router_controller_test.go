package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/router/infrared"
	"github.com/zurco34/pelican-mc-router/internal/router/mcrouter"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

func TestNewRouteControllerBuildsMCRouter(
	t *testing.T,
) {
	t.Parallel()

	controller, err := newRouteController(
		config.Config{
			Router: config.RouterConfig{
				Backend: "mc-router",
			},
			MCRouter: config.MCRouterConfig{
				APIURL: "http://mc-router:8080",
			},
		},
	)
	if err != nil {
		t.Fatalf("newRouteController() error = %v", err)
	}

	if _, ok := controller.(*mcrouter.Controller); !ok {
		t.Fatalf(
			"newRouteController() type = %T, want *mcrouter.Controller",
			controller,
		)
	}
}

func TestNewRouteControllerBuildsInfrared(
	t *testing.T,
) {
	t.Parallel()

	directory := t.TempDir()

	controller, err := newRouteController(
		config.Config{
			Router: config.RouterConfig{
				Backend: "infrared",
			},
			Infrared: config.InfraredConfig{
				ProxiesPath: filepath.Join(
					directory,
					"proxies",
				),
				ReloadMarkerPath: filepath.Join(
					directory,
					"control",
					"infrared.reload",
				),
			},
		},
	)
	if err != nil {
		t.Fatalf("newRouteController() error = %v", err)
	}

	if _, ok := controller.(*infrared.ReloadingController); !ok {
		t.Fatalf(
			"newRouteController() type = %T, want *infrared.ReloadingController",
			controller,
		)
	}
}

func TestNewRouteControllerRejectsUnsupportedBackend(
	t *testing.T,
) {
	t.Parallel()

	_, err := newRouteController(
		config.Config{
			Router: config.RouterConfig{
				Backend: "unknown",
			},
		},
	)

	if !errors.Is(err, router.ErrUnsupportedEngine) {
		t.Fatalf(
			"newRouteController() error = %v, want %v",
			err,
			router.ErrUnsupportedEngine,
		)
	}
}
