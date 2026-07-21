package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/settings"
)

type fakeSettingsStore struct {
	setupComplete bool
	setupErr      error
	settings      settings.Settings
	loadErr       error
	loadCalled    bool
}

func (f *fakeSettingsStore) IsSetupComplete() (bool, error) {
	return f.setupComplete, f.setupErr
}

func (f *fakeSettingsStore) Load() (settings.Settings, error) {
	f.loadCalled = true

	return f.settings, f.loadErr
}

func TestRefreshTaskClearsRuntimeWhenSetupIncomplete(t *testing.T) {
	store := &fakeSettingsStore{
		setupComplete: false,
	}

	manager := New()
	manager.Set(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
	)

	task := NewRefreshTask(
		store,
		5*time.Second,
		manager,
	)

	err := task.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if manager.Discovery() != nil {
		t.Fatal("runtime discovery service is not nil")
	}

	if manager.Routing() != nil {
		t.Fatal("runtime routing service is not nil")
	}
}

func TestRefreshTaskReturnsSetupStatusError(t *testing.T) {
	store := &fakeSettingsStore{
		setupErr: errors.New("database unavailable"),
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		New(),
	)

	err := task.Refresh(context.Background())
	if err == nil {
		t.Fatal("Refresh() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "determine setup status") {
		t.Fatalf(
			"Refresh() error = %q, want setup status context",
			err,
		)
	}
}

func TestRefreshTaskReturnsLoadError(t *testing.T) {
	store := &fakeSettingsStore{
		setupComplete: true,
		loadErr:       errors.New("settings unavailable"),
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		New(),
	)

	err := task.Refresh(context.Background())
	if err == nil {
		t.Fatal("Refresh() error = nil, want an error")
	}

	if !store.loadCalled {
		t.Fatal("Load() was not called")
	}

	if !strings.Contains(err.Error(), "load runtime settings") {
		t.Fatalf(
			"Refresh() error = %q, want runtime settings context",
			err,
		)
	}
}

func TestRefreshTaskRunDelegatesToRefresh(t *testing.T) {
	store := &fakeSettingsStore{
		setupComplete: false,
	}

	manager := New()
	manager.Set(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
	)

	task := NewRefreshTask(
		store,
		5*time.Second,
		manager,
	)

	err := task.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if manager.Discovery() != nil {
		t.Fatal("runtime discovery service is not nil")
	}

	if manager.Routing() != nil {
		t.Fatal("runtime routing service is not nil")
	}
}
