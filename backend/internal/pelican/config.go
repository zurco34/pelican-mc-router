package pelican

import "time"

// Config contains the configuration for the Pelican API client.
type Config struct {
	BaseURL string
	APIKey  string

	Timeout time.Duration
}
