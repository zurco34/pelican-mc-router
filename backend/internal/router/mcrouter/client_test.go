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
