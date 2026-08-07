package actionhistory

import (
	"context"
	"errors"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStoreRejectsUnboundedValues(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	for _, event := range []Event{
		{Action: "secret-value", Outcome: OutcomeSuccess},
		{Action: ActionSetup, Outcome: "unbounded-error"},
	} {
		err = store.Append(context.Background(), event)
		if !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("Append(%+v) error = %v", event, err)
		}
	}
}

func TestStoreListsFixedActionsAndOutcomes(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db, 100)
	actions := []Action{ActionBootstrap, ActionSetup, ActionSettings, ActionManualReconciliation, ActionRoutePolicy, ActionRateLimit}
	outcomes := []Outcome{OutcomeSuccess, OutcomeFailure, OutcomeDenied, OutcomeCanceled}
	for actionIndex, action := range actions {
		for outcomeIndex, outcome := range outcomes {
			occurredAt := time.Date(2026, time.January, 1, 0, actionIndex, outcomeIndex, 0, time.FixedZone("test", 3600))
			if err := store.Append(context.Background(), Event{OccurredAt: occurredAt, Action: action, Outcome: outcome}); err != nil {
				t.Fatalf("append %s/%s: %v", action, outcome, err)
			}
		}
	}
	events, err := store.List(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != len(actions)*len(outcomes) {
		t.Fatalf("stored events = %d, want %d", len(events), len(actions)*len(outcomes))
	}
	for _, event := range events {
		if !validAction(event.Action) || !validOutcome(event.Outcome) {
			t.Fatalf("invalid stored event: %+v", event)
		}
	}
	if _, err := store.List(context.Background(), -1); err != nil {
		t.Fatalf("default list: %v", err)
	}
}

func TestStoreNormalizesTimestampAndRetainsDeterministicOrder(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db, 2)
	when := time.Date(2026, time.January, 1, 12, 0, 0, 123, time.FixedZone("offset", -3600))
	for _, outcome := range []Outcome{OutcomeSuccess, OutcomeFailure, OutcomeDenied} {
		if err := store.Append(context.Background(), Event{OccurredAt: when, Action: ActionSetup, Outcome: outcome}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.List(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("retained events = %d, want 2", len(events))
	}
	if events[0].Outcome != OutcomeDenied || events[1].Outcome != OutcomeFailure {
		t.Fatalf("events = %+v, want newest first", events)
	}
	for _, event := range events {
		if event.OccurredAt.Location() != time.UTC || !event.OccurredAt.Equal(when.UTC()) {
			t.Fatalf("timestamp = %s, want normalized %s", event.OccurredAt, when.UTC())
		}
	}
}

func TestStoreAppendCancellationConcurrencyAndDatabaseFailure(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db, 100)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Append(canceled, Event{Action: ActionSetup, Outcome: OutcomeCanceled}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Append() error = %v, want context.Canceled", err)
	}

	var group sync.WaitGroup
	for i := 0; i < 24; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			err := store.Append(context.Background(), Event{Action: ActionManualReconciliation, Outcome: OutcomeSuccess})
			if err != nil {
				t.Errorf("concurrent append %d: %v", index, err)
			}
		}(i)
	}
	group.Wait()
	events, err := store.List(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 24 {
		t.Fatalf("concurrent events = %d, want 24", len(events))
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Append(context.Background(), Event{Action: ActionSetup, Outcome: OutcomeFailure}); err == nil {
		t.Fatal("Append() after database close unexpectedly succeeded")
	} else if errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Append() after database close returned validation error: %v", err)
	}
}
