// Package operationalhistory persists bounded, allowlisted operational facts.
package operationalhistory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const DefaultRetention = 1000
const MaxPageSize = 100

var ErrInvalidEvent = errors.New("invalid operational history event")

type Kind string

const (
	KindReconciliation  Kind = "reconciliation"
	KindSetup           Kind = "setup"
	KindSettings        Kind = "settings"
	KindManualReconcile Kind = "manual_reconcile"
)

type Outcome string

const (
	OutcomeNotConfigured Outcome = "not_configured"
	OutcomeSuccess       Outcome = "success"
	OutcomeFailure       Outcome = "failure"
)

type Event struct {
	ID         int64
	OccurredAt time.Time
	Kind       Kind
	Outcome    Outcome
	Desired    int
	Created    int
	Updated    int
	Deleted    int
	Changed    bool
}

type Store struct {
	db        *sql.DB
	retention int
}

func NewStore(db *sql.DB, retention ...int) *Store {
	limit := DefaultRetention
	if len(retention) > 0 && retention[0] > 0 {
		limit = retention[0]
	}
	return &Store{db: db, retention: limit}
}

func (s *Store) Append(ctx context.Context, event Event) (Event, error) {
	if err := validate(event); err != nil {
		return Event{}, err
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, fmt.Errorf("begin operational history write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `INSERT INTO operational_history (occurred_at, kind, outcome, desired, created, updated, deleted, changed) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, event.OccurredAt, event.Kind, event.Outcome, event.Desired, event.Created, event.Updated, event.Deleted, event.Changed)
	if err != nil {
		return Event{}, fmt.Errorf("insert operational history: %w", err)
	}
	event.ID, err = result.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("read operational history identifier: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM operational_history WHERE id NOT IN (SELECT id FROM operational_history ORDER BY occurred_at DESC, id DESC LIMIT ?)`, s.retention); err != nil {
		return Event{}, fmt.Errorf("prune operational history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Event{}, fmt.Errorf("commit operational history write: %w", err)
	}
	return event, nil
}

func (s *Store) List(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 || limit > MaxPageSize {
		limit = MaxPageSize
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, occurred_at, kind, outcome, desired, created, updated, deleted, changed FROM operational_history ORDER BY occurred_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list operational history: %w", err)
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.OccurredAt, &event.Kind, &event.Outcome, &event.Desired, &event.Created, &event.Updated, &event.Deleted, &event.Changed); err != nil {
			return nil, fmt.Errorf("scan operational history: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational history: %w", err)
	}
	return events, nil
}

func validate(event Event) error {
	if event.Kind != KindReconciliation && event.Kind != KindSetup && event.Kind != KindSettings && event.Kind != KindManualReconcile {
		return fmt.Errorf("history kind: %w", ErrInvalidEvent)
	}
	if event.Outcome != OutcomeNotConfigured && event.Outcome != OutcomeSuccess && event.Outcome != OutcomeFailure {
		return fmt.Errorf("history outcome: %w", ErrInvalidEvent)
	}
	if event.Desired < 0 || event.Created < 0 || event.Updated < 0 || event.Deleted < 0 {
		return fmt.Errorf("history counts: %w", ErrInvalidEvent)
	}
	return nil
}
