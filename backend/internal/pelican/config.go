package pelican

import (
	"time"

	"github.com/zurco34/pelican-mc-router/internal/retry"
)

// Config contains the configuration for the Pelican API client.
type Config struct {
	BaseURL string
	APIKey  string

	Timeout time.Duration
	Retry   retry.Config
}
