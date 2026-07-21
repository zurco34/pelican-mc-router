package setup

import (
	"context"
	"errors"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/settings"
)

var (
	errValidationFailed = errors.New("validation failed")
	errSaveFailed       = errors.New("save failed")
)

type stubSettingsStore struct {
	saved   settings.Settings
	saveErr error
}

func (s *stubSettingsStore) SaveSetup(value settings.Settings) error {
	if s.saveErr != nil {
		return s.saveErr
	}

	s.saved = value
	return nil
}

type stubPelicanValidator struct {
	baseURL string
	apiKey  string
	err     error
	called  bool
}

func (s *stubPelicanValidator) Validate(
	_ context.Context,
	baseURL string,
	apiKey string,
) error {
	s.called = true
	s.baseURL = baseURL
	s.apiKey = apiKey

	return s.err
}

func TestNewService(t *testing.T) {
	store := &stubSettingsStore{}
	validator := &stubPelicanValidator{}

	service := NewService(store, validator)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}

	if service.store == nil {
		t.Error("NewService() store = nil")
	}

	if service.validator == nil {
		t.Error("NewService() validator = nil")
	}
}
func TestServiceSetup(t *testing.T) {
	t.Run("validates and persists normalized settings", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		service := NewService(store, validator)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "  https://panel.example.com/api/application/  ",
			PelicanAPIKey: "  test-api-key  ",
			RouterDomain:  "  mc.example.com  ",
		})
		if err != nil {
			t.Fatalf("Setup() error = %v", err)
		}

		want := settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application/",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		}

		if store.saved != want {
			t.Errorf("saved settings = %#v, want %#v", store.saved, want)
		}

		if !validator.called {
			t.Fatal("validator was not called")
		}

		if validator.baseURL != want.PelicanURL {
			t.Errorf(
				"validator baseURL = %q, want %q",
				validator.baseURL,
				want.PelicanURL,
			)
		}

		if validator.apiKey != want.PelicanAPIKey {
			t.Errorf(
				"validator apiKey = %q, want %q",
				validator.apiKey,
				want.PelicanAPIKey,
			)
		}
	})

	t.Run("rejects missing router domain before validation", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		service := NewService(store, validator)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "   ",
		})
		if !errors.Is(err, ErrMissingRouterDomain) {
			t.Fatalf(
				"Setup() error = %v, want errors.Is(error, ErrMissingRouterDomain)",
				err,
			)
		}

		if validator.called {
			t.Error("validator was called for invalid input")
		}

		if store.saved != (settings.Settings{}) {
			t.Errorf("saved settings = %#v, want zero value", store.saved)
		}

	})

	t.Run("does not persist settings when validation fails", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{
			err: errValidationFailed,
		}
		service := NewService(store, validator)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application",
			PelicanAPIKey: "bad-api-key",
			RouterDomain:  "mc.example.com",
		})
		if !errors.Is(err, errValidationFailed) {
			t.Fatalf(
				"Setup() error = %v, want errors.Is(error, errValidationFailed)",
				err,
			)
		}

		if store.saved != (settings.Settings{}) {
			t.Errorf("saved settings = %#v, want zero value", store.saved)
		}

	})

	t.Run("does not mark setup complete when saving fails", func(t *testing.T) {
		store := &stubSettingsStore{
			saveErr: errSaveFailed,
		}
		validator := &stubPelicanValidator{}
		service := NewService(store, validator)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		})
		if !errors.Is(err, errSaveFailed) {
			t.Fatalf(
				"Setup() error = %v, want errors.Is(error, errSaveFailed)",
				err,
			)
		}

	})

}
