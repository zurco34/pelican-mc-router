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

type activationSynchronizer struct {
	calls   []routing.RouteSource
	after   func(int)
	failAt  int
	failErr error
}

func (s *activationSynchronizer) Sync(_ context.Context, source routing.RouteSource) error {
	s.calls = append(s.calls, source)
	if s.after != nil {
		s.after(len(s.calls))
	}
	if len(s.calls) == s.failAt {
		return s.failErr
	}
	return nil
}

type recordingReconciliationObserver struct {
	statuses []ReconciliationStatus
}

type recordingReconciliationEventRecorder struct {
	outcomes []ReconciliationOutcome
	results  []routing.ReconciliationResult
	err      error
}

func (r *recordingReconciliationEventRecorder) RecordReconciliation(
	_ context.Context,
	outcome ReconciliationOutcome,
	result routing.ReconciliationResult,
) error {
	r.outcomes = append(r.outcomes, outcome)
	r.results = append(r.results, result)
	return r.err
}

func (o *recordingReconciliationObserver) ObserveReconciliation(
	status ReconciliationStatus,
) {
	o.statuses = append(o.statuses, status)
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
		nil,
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

func TestRefreshTaskActivatePublishesOnlyAfterPersistence(t *testing.T) {
	manager := New()
	synchronizer := &recordingRouteSynchronizer{manager: manager}
	task := NewRefreshTask(
		&fakeSettingsStore{},
		5*time.Second,
		"",
		manager,
		synchronizer,
		NewReconciliationTracker(),
		nil,
	)

	persisted := false
	err := task.Activate(context.Background(), settings.Settings{
		PelicanURL:    "https://panel.example.com",
		PelicanAPIKey: "test-key",
		RouterDomain:  "mc.example.com",
	}, func() error { persisted = true; return nil })
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if synchronizer.calls != 1 {
		t.Fatalf("Sync() calls = %d, want 1", synchronizer.calls)
	}
	if synchronizer.runtimePublished {
		t.Fatal("candidate runtime was published before preparation completed")
	}
	if !persisted {
		t.Fatal("candidate settings were not persisted")
	}
	if manager.Routing() == nil {
		t.Fatal("candidate runtime was not published after persistence")
	}
	status := task.ReconciliationTracker().Snapshot()
	if status.LastOutcome == nil || *status.LastOutcome != ReconciliationOutcomeSuccess {
		t.Fatalf("candidate activation reconciliation status = %+v, want success", status)
	}
	if status.InProgress {
		t.Fatal("candidate activation reconciliation remained in progress")
	}
}

func TestRefreshTaskActivateCompensatesFreshSetupAfterSynchronizationFailure(t *testing.T) {
	manager := New()
	synchronizer := &activationSynchronizer{failAt: 1, failErr: errors.New("candidate failed")}
	task := NewRefreshTask(&fakeSettingsStore{}, 5*time.Second, "", manager, synchronizer, NewReconciliationTracker(), nil)

	err := task.Activate(context.Background(), settings.Settings{
		PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com",
	}, func() error { return nil })
	if !errors.Is(err, synchronizer.failErr) {
		t.Fatalf("Activate() error = %v, want candidate synchronization error", err)
	}
	if len(synchronizer.calls) != 2 {
		t.Fatalf("Sync() calls = %d, want candidate and compensation", len(synchronizer.calls))
	}
	if _, ok := synchronizer.calls[1].(emptyRouteSource); !ok {
		t.Fatalf("compensation source = %T, want emptyRouteSource", synchronizer.calls[1])
	}
	if manager.Routing() != nil {
		t.Fatal("failed candidate activation published runtime")
	}
}

func TestRefreshTaskActivateCompensatesAfterCancellation(t *testing.T) {
	manager := New()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	synchronizer := &activationSynchronizer{after: func(call int) {
		if call == 1 {
			cancel()
		}
	}}
	task := NewRefreshTask(&fakeSettingsStore{}, 5*time.Second, "", manager, synchronizer, NewReconciliationTracker(), nil)

	err := task.Activate(ctx, settings.Settings{
		PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com",
	}, func() error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Activate() error = %v, want context cancellation", err)
	}
	if len(synchronizer.calls) != 2 {
		t.Fatalf("Sync() calls = %d, want candidate and compensation", len(synchronizer.calls))
	}
	if _, ok := synchronizer.calls[1].(emptyRouteSource); !ok {
		t.Fatalf("compensation source = %T, want emptyRouteSource", synchronizer.calls[1])
	}
}

func TestRefreshTaskRecordsCompletedReconciliationOutcomes(t *testing.T) {
	tests := []struct {
		name           string
		store          *fakeSettingsStore
		synchronizer   *recordingRouteSynchronizer
		wantOutcome    ReconciliationOutcome
		wantRefreshErr bool
	}{
		{
			name:        "not configured",
			store:       &fakeSettingsStore{},
			wantOutcome: ReconciliationOutcomeNotConfigured,
		},
		{
			name:         "success",
			store:        &fakeSettingsStore{setupComplete: true, settings: settings.Settings{PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com"}},
			synchronizer: &recordingRouteSynchronizer{},
			wantOutcome:  ReconciliationOutcomeSuccess,
		},
		{
			name:           "failure",
			store:          &fakeSettingsStore{setupComplete: true, settings: settings.Settings{PelicanURL: "https://panel.example.com", PelicanAPIKey: "test-key", RouterDomain: "mc.example.com"}},
			synchronizer:   &recordingRouteSynchronizer{err: errors.New("unavailable")},
			wantOutcome:    ReconciliationOutcomeFailure,
			wantRefreshErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := New()
			if test.synchronizer != nil {
				test.synchronizer.manager = manager
			}
			recorder := &recordingReconciliationEventRecorder{}
			task := NewRefreshTask(test.store, 5*time.Second, "", manager, test.synchronizer, NewReconciliationTracker(), nil).
				WithReconciliationEventRecorder(recorder)
			err := task.Refresh(context.Background())
			if (err != nil) != test.wantRefreshErr {
				t.Fatalf("Refresh() error = %v, want error %t", err, test.wantRefreshErr)
			}
			if len(recorder.outcomes) != 1 || recorder.outcomes[0] != test.wantOutcome {
				t.Fatalf("recorded outcomes = %v, want [%s]", recorder.outcomes, test.wantOutcome)
			}
		})
	}
}

func TestRefreshTaskHistoryFailureDoesNotChangeReconciliationResult(t *testing.T) {
	store := &fakeSettingsStore{}
	recorder := &recordingReconciliationEventRecorder{err: errors.New("history unavailable")}
	task := NewRefreshTask(store, 5*time.Second, "", New(), nil, NewReconciliationTracker(), nil).
		WithReconciliationEventRecorder(recorder)

	if err := task.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v, want nil", err)
	}
	if len(recorder.outcomes) != 1 || recorder.outcomes[0] != ReconciliationOutcomeNotConfigured {
		t.Fatalf("recorded outcomes = %v, want not configured", recorder.outcomes)
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
		nil,
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
		nil,
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

func TestRefreshTaskCancelsWhileWaitingForAnotherRefresh(t *testing.T) {
	store := &blockingSettingsStore{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}, 1),
	}
	task := NewRefreshTask(store, 5*time.Second, "", New(), nil, NewReconciliationTracker(), nil)
	firstResult := make(chan error, 1)
	go func() { firstResult <- task.Refresh(context.Background()) }()
	select {
	case <-store.entered:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not enter the settings store")
	}

	ctx, cancel := context.WithCancel(context.Background())
	secondResult := make(chan error, 1)
	go func() { secondResult <- task.Refresh(ctx) }()
	cancel()
	select {
	case err := <-secondResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("second Refresh() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled refresh did not return")
	}

	store.release <- struct{}{}
	if err := <-firstResult; err != nil {
		t.Fatalf("first Refresh() error = %v", err)
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
		nil,
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
		nil,
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
		nil,
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
		nil,
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

func TestRefreshTaskObservesTrackerTransitions(t *testing.T) {
	tests := []struct {
		name    string
		store   *fakeSettingsStore
		syncErr error
		outcome ReconciliationOutcome
	}{
		{
			name:    "not configured",
			store:   &fakeSettingsStore{},
			outcome: ReconciliationOutcomeNotConfigured,
		},
		{
			name: "success",
			store: &fakeSettingsStore{
				setupComplete: true,
				settings: settings.Settings{
					PelicanURL:    "https://panel.example.com",
					PelicanAPIKey: "test-key",
					RouterDomain:  "mc.example.com",
				},
			},
			outcome: ReconciliationOutcomeSuccess,
		},
		{
			name: "runtime build failure",
			store: &fakeSettingsStore{
				setupErr: errors.New("settings unavailable"),
			},
			outcome: ReconciliationOutcomeFailure,
		},
		{
			name: "route synchronization failure",
			store: &fakeSettingsStore{
				setupComplete: true,
				settings: settings.Settings{
					PelicanURL:    "https://panel.example.com",
					PelicanAPIKey: "test-key",
					RouterDomain:  "mc.example.com",
				},
			},
			syncErr: errors.New("router unavailable"),
			outcome: ReconciliationOutcomeFailure,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := New()
			observer := &recordingReconciliationObserver{}
			synchronizer := &recordingRouteSynchronizer{
				manager: manager,
				err:     test.syncErr,
			}
			task := NewRefreshTask(
				test.store,
				5*time.Second,
				"",
				manager,
				synchronizer,
				NewReconciliationTracker(),
				observer,
			)

			_ = task.Refresh(context.Background())
			if len(observer.statuses) != 2 {
				t.Fatalf("observations = %d, want 2", len(observer.statuses))
			}
			if !observer.statuses[0].InProgress {
				t.Fatalf("first observation = %+v, want in progress", observer.statuses[0])
			}
			completed := observer.statuses[1]
			if completed.InProgress || completed.LastOutcome == nil || *completed.LastOutcome != test.outcome {
				t.Fatalf("completed observation = %+v, want %s", completed, test.outcome)
			}
		})
	}
}
