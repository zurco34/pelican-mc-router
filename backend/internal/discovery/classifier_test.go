package discovery

import (
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
)

func TestIsMinecraftEgg(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{
			name: "minecraft tag",
			tags: []string{"minecraft"},
			want: true,
		},
		{
			name: "case insensitive",
			tags: []string{"Minecraft"},
			want: true,
		},
		{
			name: "trims whitespace",
			tags: []string{"  minecraft  "},
			want: true,
		},
		{
			name: "minecraft among other tags",
			tags: []string{"java", "minecraft", "modded"},
			want: true,
		},
		{
			name: "non minecraft egg",
			tags: []string{"factorio"},
			want: false,
		},
		{
			name: "empty tags",
			tags: []string{},
			want: false,
		},
		{
			name: "nil tags",
			tags: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			egg := pelican.EggResource{
				Attributes: pelican.EggAttributes{
					Tags: tt.tags,
				},
			}

			got := isMinecraftEgg(egg)

			if got != tt.want {
				t.Errorf(
					"isMinecraftEgg() = %v, want %v",
					got,
					tt.want,
				)
			}
		})
	}
}
