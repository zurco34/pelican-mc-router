package pelican

import (
	"net/http"
	"time"
)

type Client struct {
	cfg        Config
	httpClient *http.Client
}

// NewClient creates a new Pelican API client.
func NewClient(cfg Config) (*Client, error) {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	if cfg.Timeout > 0 {
		httpClient.Timeout = cfg.Timeout
	}

	client := &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}

	return client, nil
}
