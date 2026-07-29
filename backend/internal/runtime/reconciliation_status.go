package runtime

import (
	"sync"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

type ReconciliationOutcome string

const (
	ReconciliationOutcomeNotConfigured ReconciliationOutcome = "not_configured"
	ReconciliationOutcomeSuccess       ReconciliationOutcome = "success"
	ReconciliationOutcomeFailure       ReconciliationOutcome = "failure"
)

// ReconciliationStatus is a read-only snapshot of refresh activity.
type ReconciliationStatus struct {
	InProgress          bool
	LastOutcome         *ReconciliationOutcome
	LastStartedAt       *time.Time
	LastCompletedAt     *time.Time
	LastSuccessAt       *time.Time
	LastDurationMS      int64
	ConsecutiveFailures int
	LastError           *string
	RouteChanges        router.ReconciliationResult
}

// ReconciliationTracker records refresh state for HTTP status consumers.
type ReconciliationTracker struct {
	mu     sync.RWMutex
	now    func() time.Time
	status ReconciliationStatus
}

func NewReconciliationTracker() *ReconciliationTracker {
	return newReconciliationTracker(time.Now)
}

func newReconciliationTracker(now func() time.Time) *ReconciliationTracker {
	return &ReconciliationTracker{now: now}
}

func (t *ReconciliationTracker) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	started := t.now().UTC()
	t.status.InProgress = true
	t.status.LastStartedAt = &started
}

func (t *ReconciliationTracker) CompleteNotConfigured() {
	t.complete(ReconciliationOutcomeNotConfigured, "", router.ReconciliationResult{})
}

func (t *ReconciliationTracker) CompleteSuccess(results ...router.ReconciliationResult) {
	t.complete(ReconciliationOutcomeSuccess, "", reconciliationResult(results))
}

func (t *ReconciliationTracker) CompleteRuntimeBuildFailure() {
	t.completeFailure("runtime build failed", router.ReconciliationResult{})
}

func (t *ReconciliationTracker) CompleteRouteSynchronizationFailure(results ...router.ReconciliationResult) {
	t.completeFailure("route synchronization failed", reconciliationResult(results))
}

func (t *ReconciliationTracker) completeFailure(summary string, result router.ReconciliationResult) {
	t.complete(ReconciliationOutcomeFailure, summary, result)
}

func (t *ReconciliationTracker) complete(
	outcome ReconciliationOutcome,
	errorSummary string,
	result router.ReconciliationResult,
) {
	t.mu.Lock()
	defer t.mu.Unlock()

	completed := t.now().UTC()
	t.status.InProgress = false
	t.status.LastOutcome = &outcome
	t.status.LastCompletedAt = &completed
	t.status.RouteChanges = result
	if t.status.LastStartedAt != nil {
		duration := completed.Sub(*t.status.LastStartedAt).Milliseconds()
		if duration > 0 {
			t.status.LastDurationMS = duration
		} else {
			t.status.LastDurationMS = 0
		}
	}

	switch outcome {
	case ReconciliationOutcomeNotConfigured:
		t.status.ConsecutiveFailures = 0
		t.status.LastError = nil
	case ReconciliationOutcomeSuccess:
		t.status.LastSuccessAt = &completed
		t.status.ConsecutiveFailures = 0
		t.status.LastError = nil
	case ReconciliationOutcomeFailure:
		t.status.ConsecutiveFailures++
		message := errorSummary
		t.status.LastError = &message
	}
}

func reconciliationResult(results []router.ReconciliationResult) router.ReconciliationResult {
	if len(results) == 0 {
		return router.ReconciliationResult{}
	}

	return results[0]
}

func (t *ReconciliationTracker) Snapshot() ReconciliationStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return copyReconciliationStatus(t.status)
}

func copyReconciliationStatus(
	status ReconciliationStatus,
) ReconciliationStatus {
	result := status
	result.LastOutcome = copyOutcome(status.LastOutcome)
	result.LastStartedAt = copyTime(status.LastStartedAt)
	result.LastCompletedAt = copyTime(status.LastCompletedAt)
	result.LastSuccessAt = copyTime(status.LastSuccessAt)
	result.LastError = copyString(status.LastError)

	return result
}

func copyOutcome(value *ReconciliationOutcome) *ReconciliationOutcome {
	if value == nil {
		return nil
	}

	copy := *value
	return &copy
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	copy := *value
	return &copy
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}

	copy := *value
	return &copy
}
