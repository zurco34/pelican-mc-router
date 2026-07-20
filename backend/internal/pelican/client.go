package pelican

import (
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
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
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		http:    httpClient,
	}

	return client, nil
}
