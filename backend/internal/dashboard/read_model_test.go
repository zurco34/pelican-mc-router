package dashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/pkg/buildinfo"
)

type fakeSetupStatus struct {
	completed bool
	err       error
}

func (f fakeSetupStatus) IsSetupComplete(context.Context) (bool, error) {
	return f.completed, f.err
}

type fakeReconciliationStatus struct{ status runtime.ReconciliationStatus }

func (f fakeReconciliationStatus) Snapshot() runtime.ReconciliationStatus {
	return f.status
}

func TestServiceSnapshotUsesCachedSafeState(t *testing.T) {
	completed := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	outcome := runtime.ReconciliationOutcomeSuccess
	errorSummary := "route synchronization failed"
	service := NewService(
		fakeSetupStatus{completed: true},
		fakeReconciliationStatus{status: runtime.ReconciliationStatus{
			LastOutcome: &outcome, LastCompletedAt: &completed,
			LastSuccessAt: &completed, LastError: &errorSummary,
			RouteChanges: router.ReconciliationResult{Desired: 2, Created: 1, Changed: true},
		}},
		buildinfo.Info{Version: "0.2.0-dev", Revision: "abc123"},
	)

	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Ready || snapshot.ReadinessReason != "ready" {
		t.Fatalf("readiness = %#v", snapshot)
	}
	if snapshot.Build.Version != "0.2.0-dev" || snapshot.Reconciliation.RouteChanges.Created != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.Reconciliation.LastError == nil || *snapshot.Reconciliation.LastError != errorSummary {
		t.Fatalf("snapshot error = %#v", snapshot.Reconciliation.LastError)
	}
}

func TestServiceSnapshotReturnsSetupError(t *testing.T) {
	want := errors.New("status unavailable")
	service := NewService(fakeSetupStatus{err: want}, fakeReconciliationStatus{}, buildinfo.Info{})
	if _, err := service.Snapshot(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Snapshot() error = %v, want %v", err, want)
	}
}
