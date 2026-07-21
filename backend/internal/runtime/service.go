package runtime

import (
	"context"

	routing "github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type DiscoveryService interface {
	Discover(context.Context) ([]models.MinecraftServer, error)
}

type RoutingService interface {
	Routes(context.Context) ([]routing.Route, error)
}
