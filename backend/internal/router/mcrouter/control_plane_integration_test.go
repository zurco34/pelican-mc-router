package mcrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

func TestControlPlaneReconciliationAgainstFakeMCRouter(t *testing.T) {
	routes := map[string]routeResponse{"stale.mc.example.com": {Backend: "10.0.0.1:25565"}}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/routes":
			_ = json.NewEncoder(w).Encode(routes)
		case r.Method == http.MethodPost && r.URL.Path == "/routes":
			var request createRouteRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			routes[request.ServerAddress] = routeResponse{Backend: request.Backend}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/routes/"):
			delete(routes, strings.TrimPrefix(r.URL.Path, "/routes/"))
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fake.Close()
	client, err := NewClient(ClientConfig{BaseURL: fake.URL})
	if err != nil {
		t.Fatal(err)
	}
	controller, err := NewController(client, WithManagedDomain("mc.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := controller.ReconcileWithResult(context.Background(), []router.Route{{ServerID: "server", Hostname: "server.mc.example.com", Backend: router.Backend{Host: "10.0.0.2", Port: 25565}}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Created != 1 || result.Deleted != 1 {
		t.Fatalf("result = %#v", result)
	}
	if _, ok := routes["server.mc.example.com"]; !ok {
		t.Fatal("desired route was not created")
	}
	if _, ok := routes["stale.mc.example.com"]; ok {
		t.Fatal("stale managed route was not deleted")
	}
}
