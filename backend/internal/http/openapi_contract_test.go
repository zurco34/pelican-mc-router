package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
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
	operationIDs := map[string]string{}
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			method = strings.ToUpper(method)
			specified[method+" "+path] = operation
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operationId", method, path)
			} else if previous, duplicate := operationIDs[operation.OperationID]; duplicate {
				t.Errorf("operationId %q is duplicated by %s and %s %s", operation.OperationID, previous, method, path)
			} else {
				operationIDs[operation.OperationID] = method + " " + path
			}
			for _, parameter := range item.Parameters {
				if parameter.Value == nil || parameter.Value.In != "path" || !strings.Contains(path, "{"+parameter.Value.Name+"}") {
					continue
				}
				if !parameter.Value.Required {
					t.Errorf("path parameter %q for %s %s is not required", parameter.Value.Name, method, path)
				}
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

func TestOpenAPIOperationalResponseConformance(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("load OpenAPI document: %v", err)
	}
	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler())
	for _, test := range []struct {
		method, path, contentType string
		status                    int
	}{
		{http.MethodGet, "/health", "text/plain", http.StatusOK},
		{http.MethodGet, "/ready", "application/json", http.StatusServiceUnavailable},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if !strings.HasPrefix(recorder.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
			}
			item := document.Paths.Find(test.path)
			if item == nil || item.GetOperation(test.method) == nil {
				t.Fatalf("OpenAPI operation missing")
			}
			if item.GetOperation(test.method).Responses.Value(strconv.Itoa(test.status)) == nil {
				t.Fatalf("OpenAPI response %d missing", test.status)
			}
		})
	}
}

func TestOpenAPIManualReconcileUnavailableResponse(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler())
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/reconcile", nil))
	if recorder.Code != http.StatusNotFound || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("status/content type = %d/%q, want 404/application/json", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	operation := document.Paths.Find("/api/v1/dashboard/reconcile").GetOperation(http.MethodPost)
	if operation.Responses.Value(strconv.Itoa(http.StatusNotFound)) == nil {
		t.Fatal("OpenAPI manual reconciliation 404 response is missing")
	}
}

func TestOpenAPISettingsBeforeSetupConflictResponse(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	operation := document.Paths.Find("/api/v1/settings").GetOperation(http.MethodPut)
	if operation.Responses.Value(strconv.Itoa(http.StatusConflict)) == nil {
		t.Fatal("OpenAPI settings update 409 response is missing")
	}
}

func TestOpenAPIRoutePolicyRequestsAreOperationSpecific(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	create := document.Paths.Find("/api/v1/route-policies").GetOperation(http.MethodPost).RequestBody.Value.Content.Get("application/json").Schema.Value
	update := document.Paths.Find("/api/v1/route-policies/{serverUUID}").GetOperation(http.MethodPut).RequestBody.Value.Content.Get("application/json").Schema.Value
	if _, ok := create.Properties["server_uuid"]; !ok || create.Properties["revision"] != nil {
		t.Fatal("create schema must require server_uuid and omit revision")
	}
	if _, ok := update.Properties["server_uuid"]; ok || update.Properties["revision"] == nil {
		t.Fatal("update schema must take server_uuid from the path and require revision")
	}
}
