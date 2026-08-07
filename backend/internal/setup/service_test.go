package setup

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/settings"
)

var (
	errValidationFailed = errors.New("validation failed")
	errSaveFailed       = errors.New("save failed")
	errRefreshFailed    = errors.New("refresh failed")
)

type stubSettingsStore struct {
	mu               sync.Mutex
	saved            settings.Settings
	staged           settings.Settings
	saveErr          error
	promoteErr       error
	setupComplete    bool
	setupCompleteErr error
}

func (s *stubSettingsStore) StageSetup(value settings.Settings) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return "", s.saveErr
	}
	s.staged = value
	return "test-generation", nil
}

func (s *stubSettingsStore) PromotePendingSetup(_ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.promoteErr != nil {
		return s.promoteErr
	}
	s.saved = s.staged
	s.setupComplete = true
	return nil
}

func (s *stubSettingsStore) Save(
	value settings.Settings,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = value

	return s.saveErr
}

func (s *stubSettingsStore) IsSetupComplete() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setupComplete, s.setupCompleteErr
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

type stubRuntimeRefresher struct {
	called bool
	err    error
}

type stubCandidateActivator struct {
	prepared  bool
	published bool
	err       error
}

func (s *stubCandidateActivator) Refresh(context.Context) error { return nil }

func (s *stubCandidateActivator) Activate(_ context.Context, _ settings.Settings, persist func() error) error {
	s.prepared = true
	if s.err != nil {
		return s.err
	}
	if err := persist(); err != nil {
		return err
	}
	s.published = true
	return nil
}

func (s *stubCandidateActivator) ActivateSetup(ctx context.Context, value settings.Settings, stage func() (string, error), promote func(string) error) error {
	generation, err := stage()
	if err != nil {
		return err
	}
	return s.Activate(ctx, value, func() error { return promote(generation) })
}

type serialSetupActivator struct {
	mu      sync.Mutex
	entered chan struct{}
	release chan struct{}
}

func (*serialSetupActivator) Refresh(context.Context) error { return nil }
func (s *serialSetupActivator) Activate(context.Context, settings.Settings, func() error) error {
	return nil
}
func (s *serialSetupActivator) ActivateSetup(_ context.Context, _ settings.Settings, stage func() (string, error), promote func(string) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	generation, err := stage()
	if err != nil {
		return err
	}
	select {
	case s.entered <- struct{}{}:
	default:
	}
	<-s.release
	return promote(generation)
}

func (s *stubRuntimeRefresher) Refresh(context.Context) error {
	s.called = true
	return s.err
}

func TestNewService(t *testing.T) {
	store := &stubSettingsStore{}
	validator := &stubPelicanValidator{}

	service := NewService(store, validator, nil)

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
	t.Run("serializes concurrent pending candidates", func(t *testing.T) {
		store := &stubSettingsStore{}
		activator := &serialSetupActivator{entered: make(chan struct{}, 1), release: make(chan struct{})}
		service := NewService(store, &stubPelicanValidator{}, activator)
		first := make(chan error, 1)
		second := make(chan error, 1)
		candidateA := settings.Settings{PelicanURL: "https://panel-a.example", PelicanAPIKey: "key-a", RouterDomain: "a.example"}
		candidateB := settings.Settings{PelicanURL: "https://panel-b.example", PelicanAPIKey: "key-b", RouterDomain: "b.example"}
		go func() { first <- service.Setup(context.Background(), candidateA) }()
		<-activator.entered
		go func() { second <- service.Setup(context.Background(), candidateB) }()
		close(activator.release)
		if err := <-first; err != nil {
			t.Fatalf("first Setup() error = %v", err)
		}
		if err := <-second; !errors.Is(err, ErrAlreadyConfigured) {
			t.Fatalf("second Setup() error = %v, want ErrAlreadyConfigured", err)
		}
		if store.saved.RouterDomain != candidateA.RouterDomain {
			t.Fatalf("saved candidate = %q, want %q", store.saved.RouterDomain, candidateA.RouterDomain)
		}
	})
	t.Run("does not persist or publish when candidate activation fails", func(t *testing.T) {
		store := &stubSettingsStore{}
		activator := &stubCandidateActivator{err: errRefreshFailed}
		service := NewService(store, &stubPelicanValidator{}, activator)
		err := service.Setup(context.Background(), settings.Settings{PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com"})
		if !errors.Is(err, errRefreshFailed) {
			t.Fatalf("Setup() error = %v", err)
		}
		if store.saved != (settings.Settings{}) {
			t.Fatal("candidate failure promoted setup")
		}
		if activator.published {
			t.Fatal("candidate failure published runtime")
		}
	})
	t.Run("promotes only after candidate activation", func(t *testing.T) {
		store := &stubSettingsStore{}
		activator := &stubCandidateActivator{}
		service := NewService(store, &stubPelicanValidator{}, activator)
		if err := service.Setup(context.Background(), settings.Settings{PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com"}); err != nil {
			t.Fatal(err)
		}
		if !activator.prepared || !activator.published {
			t.Fatal("candidate activation was not prepared and published")
		}
		if store.saved.RouterDomain == "" {
			t.Fatal("successful candidate was not persisted")
		}
	})

	t.Run("validates and persists normalized settings", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		service := NewService(store, validator, nil)
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
	t.Run("promotes setup and publishes prepared runtime", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		refresher := &stubCandidateActivator{}

		service := NewService(
			store,
			validator,
			refresher,
		)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		})
		if err != nil {
			t.Fatalf("Setup() error = %v", err)
		}

		if !refresher.prepared || !refresher.published {
			t.Fatal("prepared runtime was not published")
		}
	})

	t.Run("retains retryable setup when candidate activation fails", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		refresher := &stubCandidateActivator{
			err: errRefreshFailed,
		}

		service := NewService(
			store,
			validator,
			refresher,
		)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		})
		if !errors.Is(err, errRefreshFailed) {
			t.Fatalf(
				"Setup() error = %v, want errors.Is(error, errRefreshFailed)",
				err,
			)
		}

		if store.saved != (settings.Settings{}) {
			t.Fatal("failed first refresh promoted setup")
		}
		if store.staged == (settings.Settings{}) {
			t.Fatal("failed first refresh did not retain retryable pending setup")
		}
	})

	t.Run("updates settings and refreshes runtime", func(t *testing.T) {
		store := &stubSettingsStore{setupComplete: true}
		validator := &stubPelicanValidator{}
		refresher := &stubCandidateActivator{}

		service := NewService(
			store,
			validator,
			refresher,
		)

		err := service.Update(context.Background(), settings.Settings{
			PelicanURL:    " https://panel.example.com/api/application/ ",
			PelicanAPIKey: " test-api-key ",
			RouterDomain:  " mc.example.com ",
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}

		want := settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application/",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		}

		if store.saved != want {
			t.Fatalf(
				"saved settings = %#v, want %#v",
				store.saved,
				want,
			)
		}

		if !refresher.prepared || !refresher.published {
			t.Fatal("prepared runtime was not published")
		}
	})

	t.Run("returns an error when updating settings fails", func(t *testing.T) {
		store := &stubSettingsStore{
			saveErr: errSaveFailed, setupComplete: true,
		}
		validator := &stubPelicanValidator{}

		service := NewService(
			store,
			validator,
			nil,
		)

		err := service.Update(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application/",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		})
		if !errors.Is(err, errSaveFailed) {
			t.Fatalf(
				"Update() error = %v, want errors.Is(error, errSaveFailed)",
				err,
			)
		}
	})

	t.Run("rejects update before setup without validation", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		refresher := &stubCandidateActivator{}
		service := NewService(store, validator, refresher)
		err := service.Update(context.Background(), settings.Settings{PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com"})
		if !errors.Is(err, ErrSetupNotActive) {
			t.Fatalf("Update() error = %v", err)
		}
		if validator.called || refresher.prepared || store.saved != (settings.Settings{}) {
			t.Fatal("inactive update performed work")
		}
	})

	t.Run("rejects missing router domain before validation", func(t *testing.T) {
		store := &stubSettingsStore{}
		validator := &stubPelicanValidator{}
		service := NewService(store, validator, nil)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application/",
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
		service := NewService(store, validator, nil)

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
		service := NewService(store, validator, nil)

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

	t.Run("returns already configured when setup is already complete", func(t *testing.T) {
		store := &stubSettingsStore{
			setupComplete: true,
		}
		validator := &stubPelicanValidator{}

		service := NewService(
			store,
			validator,
			nil,
		)

		err := service.Setup(context.Background(), settings.Settings{
			PelicanURL:    "https://panel.example.com/api/application",
			PelicanAPIKey: "test-api-key",
			RouterDomain:  "mc.example.com",
		})

		if !errors.Is(err, ErrAlreadyConfigured) {
			t.Fatalf(
				"Setup() error = %v, want errors.Is(error, ErrAlreadyConfigured)",
				err,
			)
		}

		if validator.called {
			t.Fatal("validator should not be called when setup is already complete")
		}

		if store.saved != (settings.Settings{}) {
			t.Fatal("settings should not be persisted")
		}
	})

}
func TestServiceIsSetupComplete(t *testing.T) {
	store := &stubSettingsStore{
		setupComplete: true,
	}

	service := NewService(
		store,
		&stubPelicanValidator{},
		nil,
	)

	complete, err := service.IsSetupComplete(context.Background())
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}

	if !complete {
		t.Fatal("IsSetupComplete() = false, want true")
	}
}
func TestServiceIsSetupCompleteReturnsStoreError(t *testing.T) {
	store := &stubSettingsStore{
		setupCompleteErr: errors.New("database unavailable"),
	}

	service := NewService(
		store,
		&stubPelicanValidator{},
		nil,
	)

	complete, err := service.IsSetupComplete(context.Background())
	if err == nil {
		t.Fatal("IsSetupComplete() error = nil, want an error")
	}

	if complete {
		t.Fatal("IsSetupComplete() = true, want false")
	}

	const expected = "setup: determine setup status: database unavailable"

	if err.Error() != expected {
		t.Errorf(
			"IsSetupComplete() error = %q, want %q",
			err,
			expected,
		)
	}
}
