package pelican

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/retry"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	cfg        Config
	httpClient *http.Client
	retry      retry.Config
}

// NewClient creates a new Pelican Application API client.
func NewClient(cfg Config) (*Client, error) {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)

	parsedURL, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidBaseURL, cfg.BaseURL)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf(
			"%w: unsupported scheme %q",
			ErrInvalidBaseURL,
			parsedURL.Scheme,
		)
	}

	if cfg.APIKey == "" {
		return nil, ErrMissingAPIKey
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retry: cfg.Retry,
	}, nil
}
