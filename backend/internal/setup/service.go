package setup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/settings"
)

var ErrMissingRouterDomain = errors.New("setup: router domain is required")

type SettingsStore interface {
	SaveSetup(settings.Settings) error
}

type PelicanValidator interface {
	Validate(
		ctx context.Context,
		baseURL string,
		apiKey string,
	) error
}

type Service struct {
	store     SettingsStore
	validator PelicanValidator
}

func NewService(
	store SettingsStore,
	validator PelicanValidator,
) *Service {
	return &Service{
		store:     store,
		validator: validator,
	}
}
func (s *Service) Setup(
	ctx context.Context,
	input settings.Settings,
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
	if err := s.store.SaveSetup(input); err != nil {
		return fmt.Errorf("save setup: %w", err)
	}

	return nil
}
