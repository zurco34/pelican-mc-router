package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	routing "github.com/zurco34/pelican-mc-router/internal/router"
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

type recordingRouteSynchronizer struct {
	manager          *Manager
	source           routing.RouteSource
	calls            int
	runtimePublished bool
	err              error
}

func (f *recordingRouteSynchronizer) Sync(
	_ context.Context,
	source routing.RouteSource,
) error {
	f.calls++
	f.source = source
	f.runtimePublished = f.manager.Routing() != nil

	return f.err
}

func (f *blockingSettingsStore) IsSetupComplete() (bool, error) {
	f.entered <- struct{}{}
	<-f.release

	return false, nil
}

func (*blockingSettingsStore) Load() (settings.Settings, error) {
	return settings.Settings{}, nil
}

func TestRefreshTaskSynchronizesRoutesBeforePublishingRuntime(
	t *testing.T,
) {
	store := &fakeSettingsStore{
		setupComplete: true,
		settings: settings.Settings{
			PelicanURL:    "https://panel.example.com",
			PelicanAPIKey: "test-key",
			RouterDomain:  "mc.example.com",
		},
	}

	manager := New()
	synchronizer := &recordingRouteSynchronizer{
		manager: manager,
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		"",
		manager,
		synchronizer,
		NewReconciliationTracker(),
	)

	err := task.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if synchronizer.calls != 1 {
		t.Fatalf(
			"Sync() calls = %d, want 1",
			synchronizer.calls,
		)
	}

	if synchronizer.source == nil {
		t.Fatal("Sync() route source is nil")
	}

	if synchronizer.runtimePublished {
		t.Fatal("runtime was published before route synchronization")
	}

	if manager.Discovery() == nil {
		t.Fatal("runtime discovery service is nil")
	}

	if manager.Routing() == nil {
		t.Fatal("runtime routing service is nil")
	}

	status := task.ReconciliationTracker().Snapshot()
	if status.LastOutcome == nil || *status.LastOutcome != ReconciliationOutcomeSuccess {
		t.Fatalf("reconciliation status = %+v, want success", status)
	}
}

func TestRefreshTaskPreservesRuntimeWhenRouteSynchronizationFails(
	t *testing.T,
) {
	store := &fakeSettingsStore{
		setupComplete: true,
		settings: settings.Settings{
			PelicanURL:    "https://panel.example.com",
			PelicanAPIKey: "test-key",
			RouterDomain:  "mc.example.com",
		},
	}

	existingDiscovery := &fakeDiscoveryService{}
	existingRouting := &fakeRoutingService{}

	manager := New()
	manager.Set(existingDiscovery, existingRouting)

	synchronizer := &recordingRouteSynchronizer{
		manager: manager,
		err:     errors.New("proxy directory unavailable"),
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		"",
		manager,
		synchronizer,
		NewReconciliationTracker(),
	)

	err := task.Refresh(context.Background())
	if err == nil {
		t.Fatal("Refresh() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "synchronize routes") {
		t.Fatalf(
			"Refresh() error = %q, want synchronization context",
			err,
		)
	}

	if synchronizer.calls != 1 {
		t.Fatalf(
			"Sync() calls = %d, want 1",
			synchronizer.calls,
		)
	}

	if manager.Discovery() != existingDiscovery {
		t.Fatal(
			"failed synchronization replaced the existing discovery service",
		)
	}

	if manager.Routing() != existingRouting {
		t.Fatal(
			"failed synchronization replaced the existing routing service",
		)
	}

	status := task.ReconciliationTracker().Snapshot()
	if status.LastOutcome == nil || *status.LastOutcome != ReconciliationOutcomeFailure || status.LastError == nil || *status.LastError != "route synchronization failed" {
		t.Fatalf("reconciliation status = %+v, want route synchronization failure", status)
	}
}

func TestRefreshTaskSerializesConcurrentRefreshes(t *testing.T) {
	store := &blockingSettingsStore{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}, 2),
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		"",
		New(),
		nil,
		NewReconciliationTracker(),
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
		"",
		manager,
		nil,
		NewReconciliationTracker(),
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

	status := task.ReconciliationTracker().Snapshot()
	if status.LastOutcome == nil || *status.LastOutcome != ReconciliationOutcomeNotConfigured || status.LastError != nil || status.ConsecutiveFailures != 0 {
		t.Fatalf("reconciliation status = %+v, want not configured", status)
	}
}

func TestRefreshTaskReturnsSetupStatusError(t *testing.T) {
	store := &fakeSettingsStore{
		setupErr: errors.New("database unavailable"),
	}

	task := NewRefreshTask(
		store,
		5*time.Second,
		"",
		New(),
		nil,
		NewReconciliationTracker(),
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

	status := task.ReconciliationTracker().Snapshot()
	if status.LastOutcome == nil || *status.LastOutcome != ReconciliationOutcomeFailure || status.LastError == nil || *status.LastError != "runtime build failed" {
		t.Fatalf("reconciliation status = %+v, want runtime build failure", status)
	}
}

func TestRefreshTaskReturnsLoadError(t *testing.T) {
	store := &fakeSettingsStore{
		setupComplete: true,
		loadErr:       errors.New("settings unavailable"),
	}

	discoveryService := &fakeDiscoveryService{}
	routingService := &fakeRoutingService{}

	manager := New()
	manager.Set(
		discoveryService,
		routingService,
	)

	task := NewRefreshTask(
		store,
		5*time.Second,
		"",
		manager,
		nil,
		NewReconciliationTracker(),
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

	if manager.Discovery() != discoveryService {
		t.Fatal(
			"failed refresh replaced the existing discovery service",
		)
	}

	if manager.Routing() != routingService {
		t.Fatal(
			"failed refresh replaced the existing routing service",
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
		"",
		manager,
		nil,
		NewReconciliationTracker(),
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
