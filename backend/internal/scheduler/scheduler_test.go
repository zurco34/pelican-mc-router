package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

type taskFunc func(context.Context) error

func (f taskFunc) Run(ctx context.Context) error {
	return f(ctx)
}

func TestTickerRunRejectsInvalidInterval(t *testing.T) {
	ticker := NewTicker()

	err := ticker.Run(
		context.Background(),
		0,
		taskFunc(func(context.Context) error {
			return nil
		}),
	)

	if !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf(
			"Run() error = %v, want errors.Is(error, ErrInvalidInterval)",
			err,
		)
	}
}

func TestTickerRunRejectsNilTask(t *testing.T) {
	ticker := NewTicker()

	err := ticker.Run(
		context.Background(),
		time.Second,
		nil,
	)

	if !errors.Is(err, ErrTaskRequired) {
		t.Fatalf(
			"Run() error = %v, want errors.Is(error, ErrTaskRequired)",
			err,
		)
	}
}

func TestTickerRunExecutesTask(t *testing.T) {
	ticker := NewTicker()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := make(chan struct{}, 1)
	done := make(chan error, 1)

	task := taskFunc(func(context.Context) error {
		select {
		case called <- struct{}{}:
		default:
		}

		cancel()

		return nil
	})

	go func() {
		done <- ticker.Run(
			ctx,
			time.Millisecond,
			task,
		)
	}()

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("scheduled task was not executed")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}

	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}

func TestTickerRunStopsWhenContextIsCancelled(t *testing.T) {
	ticker := NewTicker()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ticker.Run(
		ctx,
		time.Hour,
		taskFunc(func(context.Context) error {
			t.Fatal("task should not run after context cancellation")
			return nil
		}),
	)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

func TestTickerRunReturnsTaskError(t *testing.T) {
	ticker := NewTicker()

	errTaskFailed := errors.New("task failed")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	err := ticker.Run(
		ctx,
		time.Millisecond,
		taskFunc(func(context.Context) error {
			return errTaskFailed
		}),
	)

	if !errors.Is(err, errTaskFailed) {
		t.Fatalf(
			"Run() error = %v, want errors.Is(error, errTaskFailed)",
			err,
		)
	}
}
