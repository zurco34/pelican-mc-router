package setup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
)

type stubPelicanClientFactory struct {
	client PelicanNodeLister
	err    error
	config pelican.Config
}

func (f *stubPelicanClientFactory) New(
	config pelican.Config,
) (PelicanNodeLister, error) {
	f.config = config
	return f.client, f.err
}

type stubPelicanNodeLister struct {
	err error
}

func (s stubPelicanNodeLister) ListNodes(
	context.Context,
) ([]pelican.NodeResource, error) {
	return nil, s.err
}

func TestPelicanValidatorValidate(t *testing.T) {
	t.Run("validates credentials using node list", func(t *testing.T) {
		factory := &stubPelicanClientFactory{
			client: stubPelicanNodeLister{},
		}

		validator := NewPelicanValidator(factory, 5*time.Second)

		err := validator.Validate(
			context.Background(),
			"https://panel.example.com/api/application",
			"test-api-key",
		)
		if err != nil {
			t.Fatalf("Validate() error = %v", err)
		}

		if factory.config.BaseURL != "https://panel.example.com/api/application" {
			t.Errorf(
				"BaseURL = %q, want %q",
				factory.config.BaseURL,
				"https://panel.example.com/api/application",
			)
		}

		if factory.config.APIKey != "test-api-key" {
			t.Errorf(
				"APIKey = %q, want %q",
				factory.config.APIKey,
				"test-api-key",
			)
		}

		if factory.config.Timeout != 5*time.Second {
			t.Errorf(
				"Timeout = %v, want %v",
				factory.config.Timeout,
				5*time.Second,
			)
		}
	})

	t.Run("returns client construction error", func(t *testing.T) {
		factory := &stubPelicanClientFactory{
			err: pelican.ErrInvalidBaseURL,
		}

		validator := NewPelicanValidator(factory, 0)

		err := validator.Validate(
			context.Background(),
			"invalid",
			"test-api-key",
		)
		if !errors.Is(err, pelican.ErrInvalidBaseURL) {
			t.Fatalf(
				"Validate() error = %v, want errors.Is(error, ErrInvalidBaseURL)",
				err,
			)
		}
	})

	t.Run("returns connectivity error", func(t *testing.T) {
		factory := &stubPelicanClientFactory{
			client: stubPelicanNodeLister{
				err: pelican.ErrUnauthorized,
			},
		}

		validator := NewPelicanValidator(factory, 0)

		err := validator.Validate(
			context.Background(),
			"https://panel.example.com/api/application",
			"bad-api-key",
		)
		if !errors.Is(err, pelican.ErrUnauthorized) {
			t.Fatalf(
				"Validate() error = %v, want errors.Is(error, ErrUnauthorized)",
				err,
			)
		}
	})
}
