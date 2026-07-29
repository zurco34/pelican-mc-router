package api

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
)

func TestOpenAPIContractMatchesRegisteredRoutes(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI document: %v", err)
	}

	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler())
	registered := map[string]bool{}
	routes, ok := server.Router().(chi.Routes)
	if !ok {
		t.Fatal("server router does not expose registered routes")
	}
	if err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		registered[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("walk registered routes: %v", err)
	}

	specified := map[string]*openapi3.Operation{}
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			method = strings.ToUpper(method)
			specified[method+" "+path] = operation
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operationId", method, path)
			}
		}
	}
	for route := range registered {
		if _, ok := specified[route]; !ok {
			t.Errorf("registered route %s is absent from OpenAPI", route)
		}
	}
	for route, operation := range specified {
		if !registered[route] {
			t.Errorf("OpenAPI route %s is not registered", route)
		}
		if route != "GET /health" && route != "GET /ready" && route != "GET /metrics" && (operation.Security == nil || len(*operation.Security) == 0) {
			t.Errorf("management route %s has no security requirement", route)
		}
	}
}
