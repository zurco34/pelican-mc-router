// Package actionhistory persists bounded, non-identifying sensitive-action facts.
package actionhistory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

const DefaultRetention = 1000

var ErrInvalidEvent = errors.New("invalid sensitive action event")

type Action string

const (
	ActionBootstrap            Action = "bootstrap_authorization"
	ActionSetup                Action = "setup"
	ActionSettings             Action = "settings_update"
	ActionManualReconciliation Action = "manual_reconciliation"
	ActionRoutePolicy          Action = "route_policy_mutation"
	ActionRateLimit            Action = "rate_limit_rejection"
)

type Outcome string

const (
	OutcomeSuccess  Outcome = "success"
	OutcomeFailure  Outcome = "failure"
	OutcomeDenied   Outcome = "denied"
	OutcomeCanceled Outcome = "canceled"
)

type Event struct {
	OccurredAt time.Time
	Action     Action
	Outcome    Outcome
}
type Store struct {
	db        *sql.DB
	retention int
	appendMu  sync.Mutex
}

func NewStore(db *sql.DB, retention ...int) *Store {
	limit := DefaultRetention
	if len(retention) > 0 && retention[0] > 0 {
		limit = retention[0]
	}
	return &Store{db: db, retention: limit}
}
func (s *Store) Append(ctx context.Context, event Event) error {
	if !validAction(event.Action) || !validOutcome(event.Outcome) {
		return ErrInvalidEvent
	}
	s.appendMu.Lock()
	defer s.appendMu.Unlock()
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sensitive action history: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO sensitive_action_history (occurred_at,action,outcome) VALUES (?,?,?)`, event.OccurredAt, event.Action, event.Outcome); err != nil {
		return fmt.Errorf("insert sensitive action history: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sensitive_action_history WHERE id NOT IN (SELECT id FROM sensitive_action_history ORDER BY occurred_at DESC,id DESC LIMIT ?)`, s.retention); err != nil {
		return fmt.Errorf("prune sensitive action history: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit sensitive action history: %w", err)
	}
	return nil
}

func (s *Store) List(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT occurred_at, action, outcome FROM sensitive_action_history ORDER BY occurred_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list sensitive action history: %w", err)
	}
	defer rows.Close()
	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.OccurredAt, &event.Action, &event.Outcome); err != nil {
			return nil, fmt.Errorf("scan sensitive action history: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
func validAction(v Action) bool {
	return v == ActionBootstrap || v == ActionSetup || v == ActionSettings || v == ActionManualReconciliation || v == ActionRoutePolicy || v == ActionRateLimit
}
func validOutcome(v Outcome) bool {
	return v == OutcomeSuccess || v == OutcomeFailure || v == OutcomeDenied || v == OutcomeCanceled
}
