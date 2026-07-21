package setup

import "github.com/zurco34/pelican-mc-router/internal/pelican"

type DefaultPelicanClientFactory struct{}

func (DefaultPelicanClientFactory) New(
	config pelican.Config,
) (PelicanNodeLister, error) {
	return pelican.NewClient(config)
}
