package discovery

import (
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
)

const minecraftTag = "minecraft"

func isMinecraftEgg(egg pelican.EggResource) bool {
	for _, tag := range egg.Attributes.Tags {
		if strings.EqualFold(strings.TrimSpace(tag), minecraftTag) {
			return true
		}
	}

	return false
}
