package discovery

import (
	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

func mapMinecraftServer(
	server pelican.ServerResource,
) models.MinecraftServer {
	return models.MinecraftServer{
		ID:           server.Attributes.ID,
		UUID:         server.Attributes.UUID,
		Identifier:   server.Attributes.Identifier,
		Name:         server.Attributes.Name,
		NodeID:       server.Attributes.Node,
		EggID:        server.Attributes.Egg,
		AllocationID: server.Attributes.Allocation,
		Suspended:    server.Attributes.Suspended,
		Status:       server.Attributes.Status,
	}
}
