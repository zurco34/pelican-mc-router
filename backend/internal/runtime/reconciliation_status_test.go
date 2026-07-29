package runtime

import (
	"sync"
	"testing"
	"time"
)

func TestReconciliationTrackerInitialSnapshot(t *testing.T) {
	status := NewReconciliationTracker().Snapshot()
	if status.InProgress || status.LastOutcome != nil || status.LastStartedAt != nil || status.LastCompletedAt != nil || status.LastSuccessAt != nil || status.LastDurationMS != 0 || status.ConsecutiveFailures != 0 || status.LastError != nil {
		t.Fatalf("initial status = %+v", status)
	}
}

func TestReconciliationTrackerTransitions(t *testing.T) {
	started := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	completed := started.Add(125 * time.Millisecond)
	restarted := completed.Add(time.Second)
	failed := restarted.Add(25 * time.Millisecond)
	times := []time.Time{started, completed, restarted, failed}
	index := 0
	tracker := newReconciliationTracker(func() time.Time { value := times[index]; index++; return value })

	tracker.Start()
	inProgress := tracker.Snapshot()
	if !inProgress.InProgress || inProgress.LastStartedAt == nil || !inProgress.LastStartedAt.Equal(started) || inProgress.LastOutcome != nil {
		t.Fatalf("in-progress status = %+v", inProgress)
	}
	tracker.CompleteSuccess()
	success := tracker.Snapshot()
	if success.LastOutcome == nil || *success.LastOutcome != ReconciliationOutcomeSuccess || success.LastSuccessAt == nil || !success.LastSuccessAt.Equal(completed) || success.LastDurationMS != 125 {
		t.Fatalf("success status = %+v", success)
	}

	tracker.Start()
	duringNextRefresh := tracker.Snapshot()
	if !duringNextRefresh.InProgress || duringNextRefresh.LastOutcome == nil || *duringNextRefresh.LastOutcome != ReconciliationOutcomeSuccess {
		t.Fatalf("next refresh status = %+v", duringNextRefresh)
	}
	tracker.CompleteRouteSynchronizationFailure()
	failure := tracker.Snapshot()
	if failure.LastOutcome == nil || *failure.LastOutcome != ReconciliationOutcomeFailure || failure.LastSuccessAt == nil || !failure.LastSuccessAt.Equal(completed) || failure.ConsecutiveFailures != 1 || failure.LastDurationMS != 25 || failure.LastError == nil || *failure.LastError != "route synchronization failed" {
		t.Fatalf("failure status = %+v", failure)
	}
}

func TestReconciliationTrackerFailureRecoveryAndNotConfigured(t *testing.T) {
	now := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	tracker := newReconciliationTracker(func() time.Time { now = now.Add(time.Millisecond); return now })

	tracker.Start()
	tracker.CompleteRuntimeBuildFailure()
	firstFailure := tracker.Snapshot()
	if firstFailure.LastSuccessAt != nil || firstFailure.ConsecutiveFailures != 1 || firstFailure.LastError == nil || *firstFailure.LastError != "runtime build failed" {
		t.Fatalf("first failure = %+v", firstFailure)
	}
	tracker.Start()
	tracker.CompleteRouteSynchronizationFailure()
	if got := tracker.Snapshot(); got.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive failures = %d, want 2", got.ConsecutiveFailures)
	}
	tracker.Start()
	tracker.CompleteSuccess()
	success := tracker.Snapshot()
	if success.ConsecutiveFailures != 0 || success.LastError != nil || success.LastSuccessAt == nil {
		t.Fatalf("recovered status = %+v", success)
	}
	lastSuccess := *success.LastSuccessAt
	tracker.Start()
	tracker.CompleteNotConfigured()
	notConfigured := tracker.Snapshot()
	if notConfigured.LastOutcome == nil || *notConfigured.LastOutcome != ReconciliationOutcomeNotConfigured || notConfigured.ConsecutiveFailures != 0 || notConfigured.LastError != nil || notConfigured.LastSuccessAt == nil || !notConfigured.LastSuccessAt.Equal(lastSuccess) {
		t.Fatalf("not configured status = %+v", notConfigured)
	}
}

func TestReconciliationTrackerRepeatedSuccess(t *testing.T) {
	now := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	tracker := newReconciliationTracker(func() time.Time { now = now.Add(time.Millisecond); return now })
	tracker.Start()
	tracker.CompleteSuccess()
	firstSuccess := *tracker.Snapshot().LastSuccessAt
	tracker.Start()
	tracker.CompleteSuccess()
	secondSuccess := tracker.Snapshot()
	if secondSuccess.LastSuccessAt == nil || !secondSuccess.LastSuccessAt.After(firstSuccess) || secondSuccess.ConsecutiveFailures != 0 || secondSuccess.LastError != nil {
		t.Fatalf("repeated success status = %+v", secondSuccess)
	}
}

func TestReconciliationTrackerClampsNegativeDuration(t *testing.T) {
	times := []time.Time{time.Date(2026, 7, 29, 6, 45, 1, 0, time.UTC), time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)}
	index := 0
	tracker := newReconciliationTracker(func() time.Time { value := times[index]; index++; return value })
	tracker.Start()
	tracker.CompleteSuccess()
	if got := tracker.Snapshot().LastDurationMS; got != 0 {
		t.Fatalf("duration = %d, want 0", got)
	}
}

func TestReconciliationTrackerSnapshotIsolation(t *testing.T) {
	now := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	tracker := newReconciliationTracker(func() time.Time { now = now.Add(time.Millisecond); return now })
	tracker.Start()
	tracker.CompleteSuccess()
	tracker.Start()
	tracker.CompleteRuntimeBuildFailure()

	snapshot := tracker.Snapshot()
	changedTime := time.Time{}
	*snapshot.LastOutcome = ReconciliationOutcomeNotConfigured
	*snapshot.LastStartedAt = changedTime
	*snapshot.LastCompletedAt = changedTime
	*snapshot.LastSuccessAt = changedTime
	*snapshot.LastError = "secret value"
	got := tracker.Snapshot()
	if got.LastOutcome == nil || *got.LastOutcome != ReconciliationOutcomeFailure || got.LastStartedAt.Equal(changedTime) || got.LastCompletedAt.Equal(changedTime) || got.LastSuccessAt.Equal(changedTime) || got.LastError == nil || *got.LastError != "runtime build failed" {
		t.Fatalf("internal state was mutated through snapshot: %+v", got)
	}
}

func TestReconciliationTrackerConcurrentSnapshots(t *testing.T) {
	tracker := NewReconciliationTracker()
	done := make(chan struct{})
	var readers sync.WaitGroup
	for range 10 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
					_ = tracker.Snapshot()
				}
			}
		}()
	}
	for range 20 {
		tracker.Start()
		tracker.CompleteSuccess()
	}
	close(done)
	readers.Wait()
}
