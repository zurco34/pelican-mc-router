package setup

import "github.com/zurco34/pelican-mc-router/internal/pelican"

type PelicanClientFactoryFunc func(
	pelican.Config,
) (PelicanNodeLister, error)

func (f PelicanClientFactoryFunc) New(
	cfg pelican.Config,
) (PelicanNodeLister, error) {
	return f(cfg)
}
