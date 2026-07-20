package pelican

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDo(t *testing.T) {
	t.Run("sends authenticated request and decodes response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}

				if r.URL.Path != "/api/application/servers" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Errorf("unexpected authorization header: %q", got)
				}

				if got := r.Header.Get("Accept"); got != "application/json" {
					t.Errorf("unexpected Accept header: %q", got)
				}

				if got := r.Header.Get("User-Agent"); got != userAgent {
					t.Errorf("unexpected User-Agent header: %q", got)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"object":"list"}`))
			},
		))
		defer server.Close()

		client, err := NewClient(Config{
			BaseURL: server.URL + "/api/application",
			APIKey:  "test-token",
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		var response struct {
			Object string `json:"object"`
		}

		err = client.do(
			context.Background(),
			http.MethodGet,
			"/servers",
			nil,
			&response,
		)
		if err != nil {
			t.Fatalf("do() error = %v", err)
		}

		if response.Object != "list" {
			t.Fatalf("unexpected object: %q", response.Object)
		}
	})

	t.Run("encodes JSON request body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Content-Type"); got != "application/json" {
					t.Errorf("unexpected Content-Type header: %q", got)
				}

				var requestBody struct {
					Name string `json:"name"`
				}

				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}

				if requestBody.Name != "example" {
					t.Errorf("unexpected name: %q", requestBody.Name)
				}

				w.WriteHeader(http.StatusNoContent)
			},
		))
		defer server.Close()

		client, err := NewClient(Config{
			BaseURL: server.URL + "/api/application",
			APIKey:  "test-token",
		})
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		err = client.do(
			context.Background(),
			http.MethodPost,
			"/servers",
			struct {
				Name string `json:"name"`
			}{
				Name: "example",
			},
			nil,
		)
		if err != nil {
			t.Fatalf("do() error = %v", err)
		}
	})

	tests := []struct {
		name       string
		statusCode int
		expected   error
	}{
		{
			name:       "maps unauthorized response",
			statusCode: http.StatusUnauthorized,
			expected:   ErrUnauthorized,
		},
		{
			name:       "maps forbidden response",
			statusCode: http.StatusForbidden,
			expected:   ErrUnauthorized,
		},
		{
			name:       "maps not found response",
			statusCode: http.StatusNotFound,
			expected:   ErrNotFound,
		},
		{
			name:       "maps unexpected response",
			statusCode: http.StatusInternalServerError,
			expected:   ErrUnexpected,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(test.statusCode)
				},
			))
			defer server.Close()

			client, err := NewClient(Config{
				BaseURL: server.URL + "/api/application",
				APIKey:  "test-token",
			})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			err = client.do(
				context.Background(),
				http.MethodGet,
				"/servers",
				nil,
				nil,
			)

			if !errors.Is(err, test.expected) {
				t.Fatalf(
					"expected error %v, got %v",
					test.expected,
					err,
				)
			}
		})
	}
}
