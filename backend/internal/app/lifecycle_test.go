package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeHTTPServer struct {
	listenStarted chan struct{}
	listenRelease chan struct{}
	listenResult  error
	shutdownErr   error
	shutdownBlock bool

	mu            sync.Mutex
	shutdownCalls int
	once          sync.Once
}

func newFakeHTTPServer() *fakeHTTPServer {
	return &fakeHTTPServer{
		listenStarted: make(chan struct{}),
		listenRelease: make(chan struct{}),
	}
}

func (s *fakeHTTPServer) ListenAndServe() error {
	close(s.listenStarted)
	<-s.listenRelease
	return s.listenResult
}

func (s *fakeHTTPServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.shutdownCalls++
	s.mu.Unlock()

	if s.shutdownBlock {
		<-ctx.Done()
		return ctx.Err()
	}

	s.once.Do(func() { close(s.listenRelease) })
	return s.shutdownErr
}

func (s *fakeHTTPServer) ShutdownCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdownCalls
}

func TestRunLifecycleCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := newFakeHTTPServer()
	schedulerStarted := make(chan struct{})
	schedulerStopped := make(chan struct{})

	results := make(chan error, 1)
	go func() {
		results <- runLifecycleWithTimeout(ctx, server, func(ctx context.Context) error {
			close(schedulerStarted)
			<-ctx.Done()
			close(schedulerStopped)
			return nil
		}, time.Second)
	}()

	<-server.listenStarted
	<-schedulerStarted
	cancel()

	if err := <-results; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if server.ShutdownCalls() != 1 {
		t.Fatalf("Shutdown() calls = %d, want 1", server.ShutdownCalls())
	}
	select {
	case <-schedulerStopped:
	default:
		t.Fatal("scheduler was not joined")
	}
}

func TestRunLifecycleHTTPServerFailure(t *testing.T) {
	server := newFakeHTTPServer()
	server.listenResult = errors.New("listen failed")
	close(server.listenRelease)
	schedulerStopped := make(chan struct{})

	err := runLifecycleWithTimeout(
		context.Background(),
		server,
		func(ctx context.Context) error {
			<-ctx.Done()
			close(schedulerStopped)
			return nil
		},
		time.Second,
	)
	if err == nil || !strings.Contains(err.Error(), "serve HTTP") {
		t.Fatalf("Run() error = %v, want HTTP context", err)
	}
	if server.ShutdownCalls() != 0 {
		t.Fatalf("Shutdown() calls = %d, want 0", server.ShutdownCalls())
	}
	select {
	case <-schedulerStopped:
	default:
		t.Fatal("scheduler was not joined")
	}
}

func TestRunLifecycleSchedulerFailure(t *testing.T) {
	server := newFakeHTTPServer()
	schedulerErr := errors.New("scheduler failed")
	shutdownErr := errors.New("shutdown failed")
	server.shutdownErr = shutdownErr

	err := runLifecycleWithTimeout(
		context.Background(),
		server,
		func(context.Context) error { return schedulerErr },
		time.Second,
	)
	if err == nil || !strings.Contains(err.Error(), "run runtime scheduler") {
		t.Fatalf("Run() error = %v, want scheduler context", err)
	}
	if !errors.Is(err, schedulerErr) {
		t.Fatalf("Run() error = %v, want scheduler error", err)
	}
	if !errors.Is(err, shutdownErr) {
		t.Fatalf("Run() error = %v, want joined shutdown error", err)
	}
	if server.ShutdownCalls() != 1 {
		t.Fatalf("Shutdown() calls = %d, want 1", server.ShutdownCalls())
	}
}

func TestRunLifecycleShutdownError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := newFakeHTTPServer()
	server.shutdownErr = errors.New("shutdown failed")
	schedulerStopped := make(chan struct{})

	results := make(chan error, 1)
	go func() {
		results <- runLifecycleWithTimeout(ctx, server, func(ctx context.Context) error {
			<-ctx.Done()
			close(schedulerStopped)
			return nil
		}, time.Second)
	}()
	<-server.listenStarted
	cancel()

	err := <-results
	if err == nil || !strings.Contains(err.Error(), "gracefully shut down HTTP server") {
		t.Fatalf("Run() error = %v, want shutdown context", err)
	}
	select {
	case <-schedulerStopped:
	default:
		t.Fatal("scheduler was not joined")
	}
}

func TestRunLifecycleShutdownTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := newFakeHTTPServer()
	server.shutdownBlock = true
	results := make(chan error, 1)
	go func() {
		results <- runLifecycleWithTimeout(ctx, server, func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}, 20*time.Millisecond)
	}()
	<-server.listenStarted
	cancel()

	err := <-results
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
	server.once.Do(func() { close(server.listenRelease) })
}

func TestRunLifecycleTreatsServerClosedAsNormal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := newFakeHTTPServer()
	server.listenResult = http.ErrServerClosed
	results := make(chan error, 1)
	go func() {
		results <- runLifecycleWithTimeout(ctx, server, func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}, time.Second)
	}()
	<-server.listenStarted
	cancel()

	if err := <-results; err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}
