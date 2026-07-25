package mcrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestClientListRoutes(t *testing.T) {
	expected := map[string]string{
		"survival.mc.example.com": "10.0.0.25:25565",
		"creative.mc.example.com": "10.0.0.26:25566",
	}

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodGet {
				t.Errorf(
					"request method = %q, want %q",
					request.Method,
					http.MethodGet,
				)
			}

			if request.URL.Path != "/routes" {
				t.Errorf(
					"request path = %q, want %q",
					request.URL.Path,
					"/routes",
				)
			}

			writer.Header().Set(
				"Content-Type",
				"application/json",
			)

			err := json.NewEncoder(writer).Encode(
				map[string]map[string]string{
					"survival.mc.example.com": {
						"backend":       "10.0.0.25:25565",
						"scalingTarget": "10.0.0.25:25565",
					},
					"creative.mc.example.com": {
						"backend":       "10.0.0.26:25566",
						"scalingTarget": "10.0.0.26:25566",
					},
				},
			)
			if err != nil {
				t.Errorf("encode response: %v", err)
			}
		},
	))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.ListRoutes(context.Background())
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf(
			"ListRoutes() = %#v, want %#v",
			got,
			expected,
		)
	}
}

func TestClientCreateRoute(t *testing.T) {
	const (
		hostname = "survival.mc.example.com"
		backend  = "10.0.0.25:25565"
	)

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodPost {
				t.Errorf(
					"request method = %q, want %q",
					request.Method,
					http.MethodPost,
				)
			}

			if request.URL.Path != "/routes" {
				t.Errorf(
					"request path = %q, want %q",
					request.URL.Path,
					"/routes",
				)
			}

			if contentType := request.Header.Get("Content-Type"); contentType != "application/json" {
				t.Errorf(
					"Content-Type = %q, want %q",
					contentType,
					"application/json",
				)
			}

			var requestBody struct {
				ServerAddress string `json:"serverAddress"`
				Backend       string `json:"backend"`
			}

			if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
				t.Errorf("decode request body: %v", err)
				return
			}

			if requestBody.ServerAddress != hostname {
				t.Errorf(
					"serverAddress = %q, want %q",
					requestBody.ServerAddress,
					hostname,
				)
			}

			if requestBody.Backend != backend {
				t.Errorf(
					"backend = %q, want %q",
					requestBody.Backend,
					backend,
				)
			}

			writer.WriteHeader(http.StatusCreated)
		},
	))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.CreateRoute(
		context.Background(),
		hostname,
		backend,
	)
	if err != nil {
		t.Fatalf("CreateRoute() error = %v", err)
	}
}

func TestClientDeleteRoute(t *testing.T) {
	const hostname = "survival.mc.example.com"

	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			if request.Method != http.MethodDelete {
				t.Errorf(
					"request method = %q, want %q",
					request.Method,
					http.MethodDelete,
				)
			}

			expectedPath := "/routes/" + hostname
			if request.URL.Path != expectedPath {
				t.Errorf(
					"request path = %q, want %q",
					request.URL.Path,
					expectedPath,
				)
			}

			writer.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.DeleteRoute(
		context.Background(),
		hostname,
	)
	if err != nil {
		t.Fatalf("DeleteRoute() error = %v", err)
	}
}
