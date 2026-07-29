// Package dashboard provides read-only data for the future operator dashboard.
package dashboard

import (
	"context"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/pkg/buildinfo"
)

type SetupStatus interface {
	IsSetupComplete(context.Context) (bool, error)
}

type ReconciliationStatusProvider interface {
	Snapshot() runtime.ReconciliationStatus
}

type Service struct {
	setup          SetupStatus
	reconciliation ReconciliationStatusProvider
	build          buildinfo.Info
}

type Snapshot struct {
	Build           buildinfo.Info
	SetupCompleted  bool
	Ready           bool
	ReadinessReason string
	Reconciliation  Reconciliation
}

type Reconciliation struct {
	InProgress          bool
	LastOutcome         *string
	LastStartedAt       *time.Time
	LastCompletedAt     *time.Time
	LastSuccessAt       *time.Time
	LastDurationMS      int64
	ConsecutiveFailures int
	LastError           *string
	RouteChanges        RouteChanges
}

type RouteChanges struct {
	Desired int
	Created int
	Updated int
	Deleted int
	Changed bool
}

func NewService(
	setup SetupStatus,
	reconciliation ReconciliationStatusProvider,
	build buildinfo.Info,
) *Service {
	return &Service{setup: setup, reconciliation: reconciliation, build: build}
}

func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	completed, err := s.setup.IsSetupComplete(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	status := s.reconciliation.Snapshot()
	return Snapshot{
		Build:           s.build,
		SetupCompleted:  completed,
		Ready:           readyFor(completed, status),
		ReadinessReason: readinessReasonFor(completed, status),
		Reconciliation:  reconciliationFrom(status),
	}, nil
}

func readyFor(completed bool, status runtime.ReconciliationStatus) bool {
	return completed && status.LastOutcome != nil &&
		*status.LastOutcome == runtime.ReconciliationOutcomeSuccess
}

func readinessReasonFor(
	completed bool,
	status runtime.ReconciliationStatus,
) string {
	if !completed {
		return "setup_incomplete"
	}
	if status.LastOutcome == nil ||
		*status.LastOutcome == runtime.ReconciliationOutcomeNotConfigured {
		return "reconciliation_pending"
	}
	if *status.LastOutcome == runtime.ReconciliationOutcomeFailure {
		return "reconciliation_failed"
	}
	if *status.LastOutcome == runtime.ReconciliationOutcomeSuccess {
		return "ready"
	}

	return "reconciliation_pending"
}

func reconciliationFrom(status runtime.ReconciliationStatus) Reconciliation {
	return Reconciliation{
		InProgress:          status.InProgress,
		LastOutcome:         outcome(status.LastOutcome),
		LastStartedAt:       copyTime(status.LastStartedAt),
		LastCompletedAt:     copyTime(status.LastCompletedAt),
		LastSuccessAt:       copyTime(status.LastSuccessAt),
		LastDurationMS:      status.LastDurationMS,
		ConsecutiveFailures: status.ConsecutiveFailures,
		LastError:           copyString(status.LastError),
		RouteChanges: RouteChanges{
			Desired: status.RouteChanges.Desired,
			Created: status.RouteChanges.Created,
			Updated: status.RouteChanges.Updated,
			Deleted: status.RouteChanges.Deleted,
			Changed: status.RouteChanges.Changed,
		},
	}
}

func outcome(value *runtime.ReconciliationOutcome) *string {
	if value == nil {
		return nil
	}

	result := string(*value)
	return &result
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	result := *value
	return &result
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}

	result := *value
	return &result
}
