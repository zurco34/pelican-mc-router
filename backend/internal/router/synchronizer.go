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
	_, err := s.SyncWithResult(ctx, source)
	return err
}

func (s *Synchronizer) SyncWithResult(
	ctx context.Context,
	source RouteSource,
) (ReconciliationResult, error) {
	if source == nil {
		return ReconciliationResult{}, ErrRouteSourceRequired
	}

	if err := ctx.Err(); err != nil {
		return ReconciliationResult{}, fmt.Errorf(
			"router: synchronize routes: %w",
			err,
		)
	}

	routes, err := source.Routes(ctx)
	if err != nil {
		return ReconciliationResult{}, fmt.Errorf(
			"router: generate routes: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return ReconciliationResult{}, fmt.Errorf(
			"router: synchronize routes: %w",
			err,
		)
	}

	if controller, ok := s.controller.(DiagnosticsRouteController); ok {
		result, err := controller.ReconcileWithResult(ctx, routes)
		if err != nil {
			return result, fmt.Errorf("router: reconcile routes: %w", err)
		}
		return result, nil
	}

	if err := s.controller.Reconcile(ctx, routes); err != nil {
		return ReconciliationResult{Desired: len(routes)}, fmt.Errorf(
			"router: reconcile routes: %w",
			err,
		)
	}

	return ReconciliationResult{Desired: len(routes)}, nil
}
