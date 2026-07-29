package operationalhistory

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
)

func TestReconciliationRecorderRecordsOnlyAllowlistedFacts(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	recorder := NewReconciliationRecorder(store)
	if err := recorder.RecordReconciliation(context.Background(), runtime.ReconciliationOutcomeSuccess, router.ReconciliationResult{Desired: 4, Created: 1, Updated: 2, Deleted: 1, Changed: true}); err != nil {
		t.Fatalf("RecordReconciliation() error = %v", err)
	}
	events, err := store.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if got, want := events[0], (Event{ID: events[0].ID, OccurredAt: events[0].OccurredAt, Kind: KindReconciliation, Outcome: OutcomeSuccess, Desired: 4, Created: 1, Updated: 2, Deleted: 1, Changed: true}); got != want {
		t.Fatalf("event = %+v, want %+v", got, want)
	}
	if err := recorder.RecordReconciliation(context.Background(), runtime.ReconciliationOutcome("unexpected"), router.ReconciliationResult{}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("RecordReconciliation() error = %v, want ErrInvalidEvent", err)
	}
}

func TestStoreAppendsListsAndPrunesAllowlistedEvents(t *testing.T) {
	t.Parallel()
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db, 2)
	ctx := context.Background()
	for index := 0; index < 3; index++ {
		_, err := store.Append(ctx, Event{OccurredAt: time.Unix(int64(index), 0), Kind: KindReconciliation, Outcome: OutcomeSuccess, Desired: index})
		if err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}
	events, err := store.List(ctx, 100)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(events) != 2 || events[0].Desired != 2 || events[1].Desired != 1 {
		t.Fatalf("events = %#v", events)
	}
}

func TestStoreRejectsUnboundedOrInvalidEvents(t *testing.T) {
	t.Parallel()
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = NewStore(db).Append(context.Background(), Event{Kind: "server-name", Outcome: OutcomeSuccess})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Append() error = %v", err)
	}
}
