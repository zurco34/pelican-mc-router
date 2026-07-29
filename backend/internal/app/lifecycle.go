package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const shutdownTimeout = 10 * time.Second

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func runLifecycle(
	ctx context.Context,
	server httpServer,
	runScheduler func(context.Context) error,
) error {
	return runLifecycleWithTimeout(
		ctx,
		server,
		runScheduler,
		shutdownTimeout,
	)
}

func runLifecycleWithTimeout(
	ctx context.Context,
	server httpServer,
	runScheduler func(context.Context) error,
	timeout time.Duration,
) error {
	runtimeCtx, cancelRuntime := context.WithCancel(ctx)
	defer cancelRuntime()

	serverResults := make(chan error, 1)
	schedulerResults := make(chan error, 1)

	go func() {
		serverResults <- server.ListenAndServe()
	}()
	go func() {
		schedulerResults <- runScheduler(runtimeCtx)
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("shutdown requested")
		return shutdownLifecycle(
			server,
			cancelRuntime,
			serverResults,
			schedulerResults,
			timeout,
		)

	case err := <-serverResults:
		cancelRuntime()
		cleanupErr := waitForWorkerWithTimeout(
			schedulerResults,
			timeout,
		)
		log.Info().Msg("runtime scheduler stopped")

		if err == nil || errors.Is(err, http.ErrServerClosed) {
			log.Info().Msg("HTTP server stopped")
			return cleanupErr
		}

		return joinErrors(fmt.Errorf("serve HTTP: %w", err), cleanupErr)

	case err := <-schedulerResults:
		cancelRuntime()
		shutdownErr := shutdownServer(server, serverResults, timeout)
		log.Info().Msg("runtime scheduler stopped")

		if err != nil {
			return joinErrors(
				fmt.Errorf("run runtime scheduler: %w", err),
				shutdownErr,
			)
		}

		return shutdownErr
	}
}

func waitForWorkerWithTimeout(
	results <-chan error,
	timeout time.Duration,
) error {
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancelShutdown()

	return waitForWorker(results, shutdownCtx)
}

func shutdownLifecycle(
	server httpServer,
	cancelRuntime context.CancelFunc,
	serverResults <-chan error,
	schedulerResults <-chan error,
	timeout time.Duration,
) error {
	cancelRuntime()

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancelShutdown()

	shutdownErr := shutdownServerWithContext(
		server,
		serverResults,
		shutdownCtx,
	)
	schedulerErr := waitForWorker(schedulerResults, shutdownCtx)
	log.Info().Msg("runtime scheduler stopped")
	if schedulerErr != nil && !errors.Is(schedulerErr, context.Canceled) {
		schedulerErr = fmt.Errorf(
			"wait for runtime scheduler: %w",
			schedulerErr,
		)
	} else {
		schedulerErr = nil
	}

	if shutdownErr == nil && schedulerErr == nil {
		log.Info().Msg("graceful shutdown completed")
	}

	return joinErrors(shutdownErr, schedulerErr)
}

func shutdownServer(
	server httpServer,
	serverResults <-chan error,
	timeout time.Duration,
) error {
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer cancelShutdown()

	return shutdownServerWithContext(server, serverResults, shutdownCtx)
}

func shutdownServerWithContext(
	server httpServer,
	serverResults <-chan error,
	shutdownCtx context.Context,
) error {
	shutdownErr := server.Shutdown(shutdownCtx)
	waitErr := waitForWorker(serverResults, shutdownCtx)
	if shutdownErr != nil ||
		(waitErr != nil && !errors.Is(waitErr, http.ErrServerClosed)) {
		return joinErrors(
			wrapError("gracefully shut down HTTP server", shutdownErr),
			wrapError("wait for HTTP server", normalizeServerResult(waitErr)),
		)
	}

	log.Info().Msg("HTTP server stopped")
	return nil
}

func normalizeServerResult(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func wrapError(operation string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", operation, err)
}

func joinErrors(primary error, cleanup error) error {
	if primary == nil {
		return cleanup
	}
	if cleanup == nil {
		return primary
	}

	return errors.Join(primary, cleanup)
}

func waitForWorker(results <-chan error, ctx context.Context) error {
	select {
	case err := <-results:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
