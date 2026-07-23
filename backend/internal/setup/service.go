package setup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/settings"
)

var ErrMissingRouterDomain = errors.New("setup: router domain is required")

var ErrAlreadyConfigured = errors.New(
	"setup: setup has already been completed",
)

type SettingsStore interface {
	IsSetupComplete() (bool, error)
	Save(settings.Settings) error
	SaveSetup(settings.Settings) error
}

type PelicanValidator interface {
	Validate(
		ctx context.Context,
		baseURL string,
		apiKey string,
	) error
}

type RuntimeRefresher interface {
	Refresh(context.Context) error
}

type Service struct {
	store     SettingsStore
	validator PelicanValidator
	refresher RuntimeRefresher
}

func NewService(
	store SettingsStore,
	validator PelicanValidator,
	refresher RuntimeRefresher,
) *Service {
	return &Service{
		store:     store,
		validator: validator,
		refresher: refresher,
	}
}

func (s *Service) IsSetupComplete(
	context.Context,
) (bool, error) {
	complete, err := s.store.IsSetupComplete()
	if err != nil {
		return false, fmt.Errorf(
			"setup: determine setup status: %w",
			err,
		)
	}

	return complete, nil
}

func (s *Service) saveAndRefresh(
	ctx context.Context,
	input settings.Settings,
	save func(settings.Settings) error,
) error {
	input.PelicanURL = strings.TrimSpace(input.PelicanURL)
	input.PelicanAPIKey = strings.TrimSpace(input.PelicanAPIKey)
	input.RouterDomain = strings.TrimSpace(input.RouterDomain)

	if input.RouterDomain == "" {
		return ErrMissingRouterDomain
	}

	if err := s.validator.Validate(
		ctx,
		input.PelicanURL,
		input.PelicanAPIKey,
	); err != nil {
		return fmt.Errorf("setup: validate Pelican credentials: %w", err)
	}

	if err := save(input); err != nil {
		return fmt.Errorf("save setup: %w", err)
	}

	if s.refresher != nil {
		if err := s.refresher.Refresh(ctx); err != nil {
			return fmt.Errorf("refresh runtime: %w", err)
		}
	}

	return nil
}

func (s *Service) Setup(
	ctx context.Context,
	input settings.Settings,
) error {
	completed, err := s.store.IsSetupComplete()
	if err != nil {
		return fmt.Errorf(
			"setup: determine setup status: %w",
			err,
		)
	}

	if completed {
		return ErrAlreadyConfigured
	}

	return s.saveAndRefresh(
		ctx,
		input,
		s.store.SaveSetup,
	)
}

func (s *Service) Update(
	ctx context.Context,
	input settings.Settings,
) error {
	return s.saveAndRefresh(
		ctx,
		input,
		s.store.Save,
	)
}
