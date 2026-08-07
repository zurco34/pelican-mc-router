package actionhistory

import (
	"context"
	"errors"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRejectsUnboundedValues(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = NewStore(db).Append(context.Background(), Event{Action: "secret-value", Outcome: OutcomeSuccess})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Append() error = %v", err)
	}
}

func TestStoreListsFixedActionsAndOutcomes(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db, 3)
	actions := []Action{ActionBootstrap, ActionSetup, ActionSettings, ActionManualReconciliation, ActionRoutePolicy, ActionRateLimit}
	outcomes := []Outcome{OutcomeSuccess, OutcomeFailure, OutcomeDenied, OutcomeCanceled}
	for index, action := range actions {
		if err := store.Append(context.Background(), Event{OccurredAt: time.Unix(int64(index), 0), Action: action, Outcome: outcomes[index%len(outcomes)]}); err != nil {
			t.Fatalf("append %s: %v", action, err)
		}
	}
	events, err := store.List(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("retained events = %d, want 3", len(events))
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
