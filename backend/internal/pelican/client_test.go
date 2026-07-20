package pelican

import (
	"errors"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	t.Run("creates client with valid configuration", func(t *testing.T) {
		client, err := NewClient(Config{
			BaseURL: "https://panel.example.com/api/application/",
			APIKey:  "test-token",
			Timeout: 15 * time.Second,
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		if client.cfg.BaseURL != "https://panel.example.com/api/application" {
			t.Fatalf("unexpected base URL: %q", client.cfg.BaseURL)
		}

		if client.httpClient.Timeout != 15*time.Second {
			t.Fatalf(
				"unexpected timeout: %s",
				client.httpClient.Timeout,
			)
		}
	})

	t.Run("uses default timeout", func(t *testing.T) {
		client, err := NewClient(Config{
			BaseURL: "https://panel.example.com/api/application",
			APIKey:  "test-token",
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		if client.httpClient.Timeout != defaultTimeout {
			t.Fatalf(
				"unexpected timeout: %s",
				client.httpClient.Timeout,
			)
		}
	})

	t.Run("rejects missing API key", func(t *testing.T) {
		_, err := NewClient(Config{
			BaseURL: "https://panel.example.com/api/application",
		})

		if !errors.Is(err, ErrMissingAPIKey) {
			t.Fatalf("expected ErrMissingAPIKey, got %v", err)
		}
	})

	t.Run("rejects invalid base URL", func(t *testing.T) {
		_, err := NewClient(Config{
			BaseURL: "not-a-url",
			APIKey:  "test-token",
		})

		if !errors.Is(err, ErrInvalidBaseURL) {
			t.Fatalf("expected ErrInvalidBaseURL, got %v", err)
		}
	})

	t.Run("rejects unsupported URL scheme", func(t *testing.T) {
		_, err := NewClient(Config{
			BaseURL: "ftp://panel.example.com/api/application",
			APIKey:  "test-token",
		})

		if !errors.Is(err, ErrInvalidBaseURL) {
			t.Fatalf("expected ErrInvalidBaseURL, got %v", err)
		}
	})
}
