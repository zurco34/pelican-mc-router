package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidInterval = errors.New(
		"scheduler interval must be greater than zero",
	)
	ErrTaskRequired = errors.New(
		"scheduler task is required",
	)
)

// Ticker runs a task at a fixed interval.
type Ticker struct{}

func NewTicker() *Ticker {
	return &Ticker{}
}

// Run blocks until the context is cancelled. Scheduled task failures are
// recoverable: the task is invoked again on the next normal interval. Startup
// reconciliation is intentionally performed outside the scheduler and remains
// fatal to application startup.
//
// The task is first executed after one complete interval. Initial runtime
// setup remains the responsibility of the application startup process.
func (t *Ticker) Run(
	ctx context.Context,
	interval time.Duration,
	task Task,
) error {
	if interval <= 0 {
		return ErrInvalidInterval
	}

	if task == nil {
		return ErrTaskRequired
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			if err := task.Run(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				// Task implementations own state tracking. Do not log an error
				// value here because it can carry control-plane topology.
				log.Warn().Msg("scheduled reconciliation failed")
			}
		}
	}
}
