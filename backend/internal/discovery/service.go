package discovery

import (
	"context"
	"fmt"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type PelicanClient interface {
	ListServers(context.Context) ([]pelican.ServerResource, error)
	ListEggs(context.Context) ([]pelican.EggResource, error)
}

type Service struct {
	client PelicanClient
}

func New(client PelicanClient) *Service {
	return &Service{
		client: client,
	}
}

func (s *Service) Discover(
	ctx context.Context,
) ([]models.MinecraftServer, error) {
	servers, err := s.client.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Pelican servers: %w", err)
	}

	eggs, err := s.client.ListEggs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Pelican eggs: %w", err)
	}

	eggsByID := make(map[int]pelican.EggResource, len(eggs))

	for _, egg := range eggs {
		eggsByID[egg.Attributes.ID] = egg
	}

	discovered := make([]models.MinecraftServer, 0)

	for _, server := range servers {
		egg, found := eggsByID[server.Attributes.Egg]
		if !found {
			continue
		}

		if !isMinecraftEgg(egg) {
			continue
		}

		discovered = append(discovered, mapMinecraftServer(server))
	}

	return discovered, nil
}
