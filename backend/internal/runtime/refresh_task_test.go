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

type blockingSettingsStore struct {
	entered chan struct{}
	release chan struct{}
}

func (f *blockingSettingsStore) IsSetupComplete() (bool, error) {
	f.entered <- struct{}{}
	<-f.release

	return false, nil
}

func (*blockingSettingsStore) Load() (settings.Settings, error) {
	return settings.Settings{}, nil
}

func TestRefreshTaskSerializesConcurrentRefreshes(t *testing.T) {
	store := &blockingSettingsStore{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}, 2),
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		New(),
	)

	results := make(chan error, 2)

	go func() {
		results <- task.Refresh(context.Background())
	}()

	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not enter the settings store")
	}

	secondStarted := make(chan struct{})

	go func() {
		close(secondStarted)
		results <- task.Refresh(context.Background())
	}()

	<-secondStarted

	select {
	case <-store.entered:
		store.release <- struct{}{}
		store.release <- struct{}{}

		<-results
		<-results

		t.Fatal(
			"second refresh entered the settings store " +
				"before the first refresh completed",
		)

	case <-time.After(100 * time.Millisecond):
	}

	// Allow the first refresh to complete.
	store.release <- struct{}{}

	// The second refresh should now be able to enter the store.
	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal(
			"second refresh did not start after " +
				"the first refresh completed",
		)
	}

	store.release <- struct{}{}

	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("Refresh() error = %v", err)
			}

		case <-time.After(time.Second):
			t.Fatal("Refresh() did not return")
		}
	}
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
