package operationalhistory

import (
	"context"
	"fmt"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
)

// ReconciliationRecorder adapts runtime reconciliation outcomes to the
// allowlisted operational-history schema.
type ReconciliationRecorder struct {
	store *Store
}

func NewReconciliationRecorder(store *Store) *ReconciliationRecorder {
	return &ReconciliationRecorder{store: store}
}

func (r *ReconciliationRecorder) RecordReconciliation(
	ctx context.Context,
	outcome runtime.ReconciliationOutcome,
	result router.ReconciliationResult,
) error {
	if r == nil || r.store == nil {
		return nil
	}

	eventOutcome, err := reconciliationOutcome(outcome)
	if err != nil {
		return err
	}
	_, err = r.store.Append(ctx, Event{
		Kind:    KindReconciliation,
		Outcome: eventOutcome,
		Desired: result.Desired,
		Created: result.Created,
		Updated: result.Updated,
		Deleted: result.Deleted,
		Changed: result.Changed,
	})
	if err != nil {
		return fmt.Errorf("append reconciliation history: %w", err)
	}
	return nil
}

func reconciliationOutcome(outcome runtime.ReconciliationOutcome) (Outcome, error) {
	switch outcome {
	case runtime.ReconciliationOutcomeNotConfigured:
		return OutcomeNotConfigured, nil
	case runtime.ReconciliationOutcomeSuccess:
		return OutcomeSuccess, nil
	case runtime.ReconciliationOutcomeFailure:
		return OutcomeFailure, nil
	default:
		return "", fmt.Errorf("reconciliation outcome: %w", ErrInvalidEvent)
	}
}
