package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/zurco34/pelican-mc-router/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if run(ctx, app.Run, log.Default()) != 0 {
		os.Exit(1)
	}
}

func run(ctx context.Context, runApp func(context.Context) error, logger *log.Logger) int {
	if err := runApp(ctx); err != nil {
		// Application errors can wrap URLs, credentials, and backend data. The
		// detailed cause is handled by safe lower-level logging; process output
		// stays stable and safe for supervisors and test captures.
		logger.Print("pelican mc router stopped due to an operational failure")
		return 1
	}
	return 0
}
