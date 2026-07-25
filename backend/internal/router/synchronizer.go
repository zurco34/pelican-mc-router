package router

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrRouteControllerRequired = errors.New(
		"router: route controller is required",
	)
	ErrRouteSourceRequired = errors.New(
		"router: route source is required",
	)
)

type RouteSource interface {
	Routes(context.Context) ([]Route, error)
}

type RouteController interface {
	Reconcile(context.Context, []Route) error
}

type Synchronizer struct {
	controller RouteController
}

func NewSynchronizer(
	controller RouteController,
) (*Synchronizer, error) {
	if controller == nil {
		return nil, ErrRouteControllerRequired
	}

	return &Synchronizer{
		controller: controller,
	}, nil
}

func (s *Synchronizer) Sync(
	ctx context.Context,
	source RouteSource,
) error {
	if source == nil {
		return ErrRouteSourceRequired
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"router: synchronize routes: %w",
			err,
		)
	}

	routes, err := source.Routes(ctx)
	if err != nil {
		return fmt.Errorf(
			"router: generate routes: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"router: synchronize routes: %w",
			err,
		)
	}

	if err := s.controller.Reconcile(ctx, routes); err != nil {
		return fmt.Errorf(
			"router: reconcile routes: %w",
			err,
		)
	}

	return nil
}
