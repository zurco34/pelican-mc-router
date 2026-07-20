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
	ListNodeAllocations(
		context.Context,
		int,
	) ([]pelican.AllocationResource, error)
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

	allocationLookup := make(map[int]pelican.AllocationAttributes)
	processedNodes := make(map[int]struct{})

	for _, server := range servers {
		nodeID := server.Attributes.Node

		if _, ok := processedNodes[nodeID]; ok {
			continue
		}

		allocations, err := s.client.ListNodeAllocations(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf(
				"list allocations for node %d: %w",
				nodeID,
				err,
			)
		}

		for _, allocation := range allocations {
			allocationLookup[allocation.Attributes.ID] =
				allocation.Attributes
		}

		processedNodes[nodeID] = struct{}{}
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

		allocation, found :=
			allocationLookup[server.Attributes.Allocation]
		if !found {
			return nil, fmt.Errorf(
				"allocation %d not found for server %q",
				server.Attributes.Allocation,
				server.Attributes.Name,
			)
		}

		minecraftServer := mapMinecraftServer(server)
		minecraftServer.BackendIP = allocation.IP
		minecraftServer.BackendPort = allocation.Port

		discovered = append(discovered, minecraftServer)
	}

	return discovered, nil
}
