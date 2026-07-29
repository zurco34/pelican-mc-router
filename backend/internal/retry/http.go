package retry

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

var DefaultConfig = Config{
	Attempts:       3,
	InitialBackoff: 200 * time.Millisecond,
	MaxBackoff:     2 * time.Second,
}

// Config bounds retries for idempotent HTTP requests.
type Config struct {
	Attempts       int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func Do(
	ctx context.Context,
	cfg Config,
	do func() (*http.Response, error),
) (*http.Response, error) {
	cfg = normalize(cfg)
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		response, err := do()
		if !shouldRetry(response, err) || attempt >= cfg.Attempts {
			return response, err
		}

		if response != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}

		if err := wait(ctx, backoff(cfg, attempt)); err != nil {
			return nil, err
		}
	}
}

func normalize(cfg Config) Config {
	if cfg.Attempts <= 0 || cfg.InitialBackoff <= 0 || cfg.MaxBackoff < cfg.InitialBackoff {
		return DefaultConfig
	}

	return cfg
}

func shouldRetry(response *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if response == nil {
		return false
	}

	switch response.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func backoff(cfg Config, attempt int) time.Duration {
	delay := cfg.InitialBackoff
	for retry := 1; retry < attempt && delay < cfg.MaxBackoff; retry++ {
		delay *= 2
	}
	if delay > cfg.MaxBackoff {
		delay = cfg.MaxBackoff
	}

	return delay/2 + time.Duration(rand.Int64N(int64(delay/2)+1))
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
