package retry

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDoRetriesTransientResponses(t *testing.T) {
	calls := 0
	response, err := Do(context.Background(), Config{
		Attempts:       3,
		InitialBackoff: time.Nanosecond,
		MaxBackoff:     time.Nanosecond,
	}, func() (*http.Response, error) {
		calls++
		if calls == 1 {
			return responseWithStatus(http.StatusServiceUnavailable), nil
		}
		return responseWithStatus(http.StatusOK), nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	_ = response.Body.Close()
}

func TestDoDoesNotRetryPermanentResponse(t *testing.T) {
	calls := 0
	response, err := Do(context.Background(), Config{
		Attempts:       3,
		InitialBackoff: time.Nanosecond,
		MaxBackoff:     time.Nanosecond,
	}, func() (*http.Response, error) {
		calls++
		return responseWithStatus(http.StatusBadRequest), nil
	})
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	_ = response.Body.Close()
}

func TestDoHonorsCancellationDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := Do(ctx, Config{
		Attempts:       3,
		InitialBackoff: time.Second,
		MaxBackoff:     time.Second,
	}, func() (*http.Response, error) {
		calls++
		cancel()
		return nil, errors.New("temporary failure")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context cancellation", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDoDoesNotStartCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0

	_, err := Do(ctx, DefaultConfig, func() (*http.Response, error) {
		calls++
		return responseWithStatus(http.StatusOK), nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context cancellation", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}

func responseWithStatus(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: ioNopCloser{Reader: strings.NewReader("")}}
}

type ioNopCloser struct{ *strings.Reader }

func (ioNopCloser) Close() error { return nil }
