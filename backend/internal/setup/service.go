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
	StageSetup(settings.Settings) error
	PromotePendingSetup() error
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

type CandidateRuntimeActivator interface {
	Activate(context.Context, settings.Settings, func() error) error
}

type SecretResolver interface{ Resolve(string) ([]byte, error) }

type Service struct {
	store     SettingsStore
	validator PelicanValidator
	refresher RuntimeRefresher
	resolver  SecretResolver
}

func NewService(
	store SettingsStore,
	validator PelicanValidator,
	refresher RuntimeRefresher, resolvers ...SecretResolver,
) *Service {
	var resolver SecretResolver
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	return &Service{
		store:     store,
		validator: validator,
		refresher: refresher,
		resolver:  resolver,
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

func (s *Service) prepareAndPublish(
	ctx context.Context,
	input settings.Settings,
	save func(settings.Settings) error,
) error {
	if err := s.validate(ctx, &input); err != nil {
		return err
	}
	if s.refresher == nil {
		if err := save(input); err != nil {
			return fmt.Errorf("save settings: %w", err)
		}
		return nil
	}
	activator, ok := s.refresher.(CandidateRuntimeActivator)
	if !ok {
		return errors.New("setup: candidate runtime activation is unavailable")
	}
	if err := activator.Activate(ctx, input, func() error { return save(input) }); err != nil {
		return fmt.Errorf("activate candidate runtime: %w", err)
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

	if err := s.validate(ctx, &input); err != nil {
		return err
	}
	if err := s.store.StageSetup(input); err != nil {
		return fmt.Errorf("stage setup: %w", err)
	}
	if s.refresher == nil {
		if err := s.store.PromotePendingSetup(); err != nil {
			return fmt.Errorf("promote setup: %w", err)
		}
		return nil
	}
	activator, ok := s.refresher.(CandidateRuntimeActivator)
	if !ok {
		return errors.New("setup: candidate runtime activation is unavailable")
	}
	if err := activator.Activate(ctx, input, s.store.PromotePendingSetup); err != nil {
		return fmt.Errorf("activate candidate runtime: %w", err)
	}
	return nil
}

func (s *Service) validate(ctx context.Context, input *settings.Settings) error {
	input.PelicanURL = strings.TrimSpace(input.PelicanURL)
	input.PelicanAPIKey = strings.TrimSpace(input.PelicanAPIKey)
	input.PelicanSecretName = strings.TrimSpace(input.PelicanSecretName)
	input.RouterDomain = strings.TrimSpace(input.RouterDomain)
	if input.RouterDomain == "" {
		return ErrMissingRouterDomain
	}
	key := input.PelicanAPIKey
	if input.PelicanSecretName != "" {
		if s.resolver == nil {
			return errors.New("setup: secret resolver is unavailable")
		}
		secret, err := s.resolver.Resolve(input.PelicanSecretName)
		if err != nil {
			return errors.New("setup: Pelican credential is unavailable")
		}
		key = string(secret)
		clear(secret)
	}
	if err := s.validator.Validate(ctx, input.PelicanURL, key); err != nil {
		return fmt.Errorf("setup: validate Pelican credentials: %w", err)
	}
	return nil
}

func (s *Service) Update(
	ctx context.Context,
	input settings.Settings,
) error {
	return s.prepareAndPublish(ctx, input, s.store.Save)
}
