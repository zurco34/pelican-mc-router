package scheduler

import (
	"context"
	"time"
)

// Task represents work executed by the scheduler.
type Task interface {
	Run(context.Context) error
}

// Scheduler executes a task repeatedly until the context is cancelled
// or the task returns an error.
type Scheduler interface {
	Run(context.Context, time.Duration, Task) error
}
