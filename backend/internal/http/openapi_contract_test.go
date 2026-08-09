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

	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler()).WithManagementAuthorization(fakeDashboardAuthorizer{})
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
	metrics := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("test_metric 1\n"))
	})
	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), metrics)
	for _, test := range []struct {
		method, path, contentType string
		status                    int
	}{
		{http.MethodGet, "/health", "text/plain", http.StatusOK},
		{http.MethodGet, "/ready", "application/json", http.StatusServiceUnavailable},
		{http.MethodGet, "/metrics", "text/plain", http.StatusOK},
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
	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler()).WithManagementAuthorization(fakeDashboardAuthorizer{})
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

func TestOpenAPISeparatesStrictRequestsFromAdditiveResponses(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	strict := document.Components.Schemas["SettingsRequest"].Value.AdditionalProperties
	if strict.Has == nil || *strict.Has || strict.Schema != nil {
		t.Fatal("settings requests must reject unknown fields")
	}
	for _, name := range []string{"ActionHistory", "ManualReconcile"} {
		if has := document.Components.Schemas[name].Value.AdditionalProperties.Has; has != nil && *has {
			t.Fatalf("%s response must permit additive fields", name)
		}
	}
}

func TestOpenAPIRouteAuthorizationClassesAreExplicit(t *testing.T) {
	loader := openapi3.NewLoader()
	document, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	classes := map[string]string{
		"GET /health":                                "public",
		"GET /ready":                                 "public",
		"GET /metrics":                               "public",
		"GET /api/v1/setup":                          "bootstrapBearer",
		"POST /api/v1/setup":                         "bootstrapBearer",
		"GET /api/v1/status":                         "oidcBearer",
		"GET /dashboard":                             "oidcBearer",
		"GET /api/v1/servers":                        "oidcBearer",
		"GET /api/v1/routes":                         "oidcBearer",
		"GET /api/v1/routes/preview":                 "oidcBearer",
		"GET /api/v1/route-policies":                 "oidcBearer",
		"POST /api/v1/route-policies":                "oidcBearer",
		"PUT /api/v1/route-policies/{serverUUID}":    "oidcBearer",
		"DELETE /api/v1/route-policies/{serverUUID}": "oidcBearer",
		"GET /api/v1/operational-history":            "oidcBearer",
		"GET /api/v1/action-history":                 "oidcBearer",
		"POST /api/v1/dashboard/reconcile":           "oidcBearer",
		"PUT /api/v1/settings":                       "oidcBearer",
	}
	for key, want := range classes {
		parts := strings.SplitN(key, " ", 2)
		operation := document.Paths.Find(parts[1]).GetOperation(parts[0])
		if operation == nil {
			t.Errorf("%s is absent from OpenAPI", key)
			continue
		}
		if want == "public" {
			if operation.Security != nil && len(*operation.Security) != 0 {
				t.Errorf("%s unexpectedly has security", key)
			}
			continue
		}
		if operation.Security == nil || len(*operation.Security) != 1 {
			t.Errorf("%s security = %#v, want %s", key, operation.Security, want)
			continue
		}
		if _, ok := (*operation.Security)[0][want]; !ok {
			t.Errorf("%s security = %#v, want %s", key, operation.Security, want)
		}
	}
}
