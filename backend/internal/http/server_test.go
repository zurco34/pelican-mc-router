package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/dashboard"
	"github.com/zurco34/pelican-mc-router/internal/dashboardauth"
	"github.com/zurco34/pelican-mc-router/internal/observability"
	routing "github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
	"github.com/zurco34/pelican-mc-router/pkg/buildinfo"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type fakeDiscoveryService struct {
	servers []models.MinecraftServer
	err     error
}

func (f *fakeDiscoveryService) Discover(
	context.Context,
) ([]models.MinecraftServer, error) {
	return f.servers, f.err
}

type fakeRoutingService struct {
	routes []routing.Route
	err    error
}

func (f *fakeRoutingService) Routes(
	context.Context,
) ([]routing.Route, error) {
	return f.routes, f.err
}

type fakeSetupService struct {
	completed bool
	err       error
	received  settings.Settings
}

type fakeStatusProvider struct {
	snapshots []runtime.ReconciliationStatus
	calls     int
}

type fakeDashboardAuthorizer struct {
	err         error
	operatorErr error
}

func (f fakeDashboardAuthorizer) Authorize(context.Context, *http.Request) error {
	return f.err
}

func (f fakeDashboardAuthorizer) AuthorizeOperator(context.Context, *http.Request) error {
	return f.operatorErr
}

type fakeDashboardRefresher struct {
	err   error
	calls int
}

func (f *fakeDashboardRefresher) Refresh(context.Context) error {
	f.calls++
	return f.err
}

func (f *fakeStatusProvider) Snapshot() runtime.ReconciliationStatus {
	result := f.snapshots[f.calls]
	f.calls++
	return result
}

func (f *fakeSetupService) IsSetupComplete(
	context.Context,
) (bool, error) {
	return f.completed, f.err
}

func (f *fakeSetupService) Setup(
	_ context.Context,
	setupSettings settings.Settings,
) error {
	f.received = setupSettings

	return f.err
}

func (f *fakeSetupService) Update(
	_ context.Context,
	setupSettings settings.Settings,
) error {
	f.received = setupSettings

	return f.err
}

func newTestServer(
	discovery runtime.DiscoveryService,
	routingService runtime.RoutingService,
	setupService SetupService,
) *Server {
	runtimeManager := runtime.New()
	runtimeManager.Set(
		discovery,
		routingService,
	)

	return NewServer(
		runtimeManager,
		setupService,
		runtime.NewReconciliationTracker(),
		http.NotFoundHandler(),
	)
}

func TestHealthHandler(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if got := recorder.Body.String(); got != "OK" {
		t.Errorf("response body = %q, want %q", got, "OK")
	}

	if got := recorder.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Errorf(
			"Content-Type = %q, want %q",
			got,
			"text/plain; charset=utf-8",
		)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	registry, metrics, err := observability.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	metrics.ObserveReconciliation(runtime.ReconciliationStatus{InProgress: true})
	server := NewServer(
		runtime.New(),
		&fakeSetupService{},
		runtime.NewReconciliationTracker(),
		observability.NewHandler(registry),
	)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("content type = %q", got)
	}
	body := recorder.Body.String()
	for _, metric := range []string{
		"pelican_mc_router_reconciliation_total",
		"go_goroutines",
		"process_start_time_seconds",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metric %q was not exposed", metric)
		}
	}
	if strings.Contains(body, "secret-value") {
		t.Fatalf("secret exposed in metrics: %s", body)
	}
}

func TestDashboardPageRendersCachedSafeState(t *testing.T) {
	completed := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	outcome := runtime.ReconciliationOutcomeSuccess
	secret := "dashboard-secret-bearer-value"
	server := NewServer(
		runtime.New(),
		&fakeSetupService{completed: true},
		runtime.NewReconciliationTracker(),
		http.NotFoundHandler(),
	).WithDashboard(dashboard.NewService(
		&fakeSetupService{completed: true},
		&fakeStatusProvider{snapshots: []runtime.ReconciliationStatus{{
			LastOutcome:     &outcome,
			LastCompletedAt: &completed,
			LastError:       &secret,
		}}},
		buildinfo.Info{Version: "0.2.0-dev", Revision: "abc123"},
	))

	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content type = %q", got)
	}
	body := recorder.Body.String()
	for _, want := range []string{"Pelican MC Router", "Ready", "0.2.0-dev", "abc123"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard response does not contain %q: %s", want, body)
		}
	}
	if strings.Contains(body, secret) {
		t.Fatalf("dashboard exposed secret: %s", body)
	}
}

func TestDashboardPageUnavailableDoesNotExposeSetupError(t *testing.T) {
	secret := "https://secret.example/dashboard-token"
	server := NewServer(
		runtime.New(),
		&fakeSetupService{},
		runtime.NewReconciliationTracker(),
		http.NotFoundHandler(),
	).WithDashboard(dashboard.NewService(
		&fakeSetupService{err: errors.New(secret)},
		runtime.NewReconciliationTracker(),
		buildinfo.Info{},
	))

	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "dashboard status unavailable") || strings.Contains(body, secret) {
		t.Fatalf("unexpected error response: %s", body)
	}
}

func TestDashboardPageAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		authorizer fakeDashboardAuthorizer
		wantStatus int
		wantBody   string
	}{
		{"unauthenticated", fakeDashboardAuthorizer{err: dashboardauth.ErrUnauthenticated}, http.StatusUnauthorized, "dashboard authentication required"},
		{"forbidden", fakeDashboardAuthorizer{err: dashboardauth.ErrForbidden}, http.StatusForbidden, "dashboard access denied"},
		{"authorization error", fakeDashboardAuthorizer{err: errors.New("dashboard-auth-secret")}, http.StatusUnauthorized, "dashboard authentication required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler()).WithDashboard(
				dashboard.NewService(&fakeSetupService{}, runtime.NewReconciliationTracker(), buildinfo.Info{}),
			).WithDashboardAuthorization(test.authorizer)
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if body := recorder.Body.String(); !strings.Contains(body, test.wantBody) || strings.Contains(body, "dashboard-auth-secret") {
				t.Fatalf("response = %s, want %q", body, test.wantBody)
			}
		})
	}
}

func TestDashboardManualReconcile(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		authorizer fakeDashboardAuthorizer
		refreshErr error
		wantStatus int
		wantCalls  int
		secret     string
	}{
		{name: "success", header: "1", wantStatus: http.StatusOK, wantCalls: 1},
		{name: "missing CSRF header", wantStatus: http.StatusForbidden},
		{name: "operator denied", header: "1", authorizer: fakeDashboardAuthorizer{operatorErr: dashboardauth.ErrForbidden}, wantStatus: http.StatusForbidden},
		{name: "canceled", header: "1", refreshErr: context.Canceled, wantStatus: http.StatusRequestTimeout, wantCalls: 1},
		{name: "failure hides details", header: "1", refreshErr: errors.New("manual-refresh-secret"), wantStatus: http.StatusServiceUnavailable, wantCalls: 1, secret: "manual-refresh-secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			refresher := &fakeDashboardRefresher{err: test.refreshErr}
			server := NewServer(
				runtime.New(),
				&fakeSetupService{},
				runtime.NewReconciliationTracker(),
				http.NotFoundHandler(),
			).WithDashboardAuthorization(test.authorizer).WithDashboardActions(refresher)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/reconcile", nil)
			request.Header.Set(dashboardCSRFHeader, test.header)
			recorder := httptest.NewRecorder()

			server.Router().ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if refresher.calls != test.wantCalls {
				t.Fatalf("refresh calls = %d, want %d", refresher.calls, test.wantCalls)
			}
			if test.secret != "" && strings.Contains(recorder.Body.String(), test.secret) {
				t.Fatalf("response exposed refresh error: %s", recorder.Body.String())
			}
		})
	}
}

type testReconciliationResponse struct {
	InProgress          bool    `json:"in_progress"`
	LastOutcome         *string `json:"last_outcome"`
	LastStartedAt       *string `json:"last_started_at"`
	LastCompletedAt     *string `json:"last_completed_at"`
	LastSuccessAt       *string `json:"last_success_at"`
	LastDurationMS      int64   `json:"last_duration_ms"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	LastError           *string `json:"last_error"`
	RouteChanges        struct {
		Desired int  `json:"desired"`
		Created int  `json:"created"`
		Updated int  `json:"updated"`
		Deleted int  `json:"deleted"`
		Changed bool `json:"changed"`
	} `json:"route_changes"`
}

type testStatusResponse struct {
	Build           buildinfo.Info             `json:"build"`
	SetupCompleted  bool                       `json:"setup_completed"`
	Ready           bool                       `json:"ready"`
	ReadinessReason string                     `json:"readiness_reason"`
	Reconciliation  testReconciliationResponse `json:"reconciliation"`
}

func TestStatusEndpointIncludesBuildIdentity(t *testing.T) {
	server := NewServer(
		runtime.New(),
		&fakeSetupService{},
		runtime.NewReconciliationTracker(),
		http.NotFoundHandler(),
		buildinfo.Info{Version: "0.1.3", Revision: "abc123"},
	)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body testStatusResponse
	decodeStrictJSON(t, recorder.Body.Bytes(), &body)
	if body.Build != (buildinfo.Info{Version: "0.1.3", Revision: "abc123"}) {
		t.Fatalf("build = %#v", body.Build)
	}
}

func TestStatusEndpoint(t *testing.T) {
	started := time.Date(2026, 7, 29, 6, 45, 0, 0, time.UTC)
	completed := started.Add(125 * time.Millisecond)
	success := runtime.ReconciliationOutcomeSuccess
	failure := runtime.ReconciliationOutcomeFailure
	sanitized := "route synchronization failed"

	tests := []struct {
		name           string
		setup          *fakeSetupService
		status         runtime.ReconciliationStatus
		wantHTTPStatus int
		wantReady      bool
		wantReason     string
	}{
		{"before setup", &fakeSetupService{}, runtime.ReconciliationStatus{}, http.StatusOK, false, "setup_incomplete"},
		{"pending", &fakeSetupService{completed: true}, runtime.ReconciliationStatus{}, http.StatusOK, false, "reconciliation_pending"},
		{"success", &fakeSetupService{completed: true}, runtime.ReconciliationStatus{LastOutcome: &success, LastStartedAt: &started, LastCompletedAt: &completed, LastSuccessAt: &completed, LastDurationMS: 125}, http.StatusOK, true, "ready"},
		{"success in progress", &fakeSetupService{completed: true}, runtime.ReconciliationStatus{InProgress: true, LastOutcome: &success, LastStartedAt: &completed, LastCompletedAt: &completed, LastSuccessAt: &completed}, http.StatusOK, true, "ready"},
		{"failure", &fakeSetupService{completed: true}, runtime.ReconciliationStatus{LastOutcome: &failure, LastStartedAt: &started, LastCompletedAt: &completed, LastSuccessAt: &started, LastDurationMS: 125, ConsecutiveFailures: 2, LastError: &sanitized}, http.StatusOK, false, "reconciliation_failed"},
		{"setup error", &fakeSetupService{err: errors.New("https://secret.example/api-key")}, runtime.ReconciliationStatus{}, http.StatusInternalServerError, false, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &fakeStatusProvider{snapshots: []runtime.ReconciliationStatus{test.status}}
			server := NewServer(runtime.New(), test.setup, provider, http.NotFoundHandler())
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
			raw := recorder.Body.Bytes()
			if recorder.Code != test.wantHTTPStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantHTTPStatus)
			}
			if strings.Contains(string(raw), "secret.example") || strings.Contains(string(raw), "api-key") {
				t.Fatalf("secret exposed in response: %s", raw)
			}
			if test.setup.err != nil {
				var body struct {
					Error string `json:"error"`
				}
				decodeStrictJSON(t, raw, &body)
				if body.Error != "failed to get application status" {
					t.Fatalf("error = %q", body.Error)
				}
				return
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type = %q", got)
			}
			var body testStatusResponse
			decodeStrictJSON(t, raw, &body)
			if body.Ready != test.wantReady || body.ReadinessReason != test.wantReason {
				t.Fatalf("response = %+v", body)
			}
			if test.name == "success" && (body.Reconciliation.LastStartedAt == nil || *body.Reconciliation.LastStartedAt != "2026-07-29T06:45:00Z" || body.Reconciliation.LastCompletedAt == nil || *body.Reconciliation.LastCompletedAt != "2026-07-29T06:45:00.125Z") {
				t.Fatalf("timestamps = %+v", body.Reconciliation)
			}
			if test.name == "failure" && (body.Reconciliation.LastError == nil || *body.Reconciliation.LastError != sanitized || body.Reconciliation.LastSuccessAt == nil || *body.Reconciliation.LastSuccessAt != "2026-07-29T06:45:00Z") {
				t.Fatalf("failure response = %+v", body.Reconciliation)
			}
		})
	}
}

func TestStatusEndpointUsesOneSnapshot(t *testing.T) {
	success := runtime.ReconciliationOutcomeSuccess
	failure := runtime.ReconciliationOutcomeFailure
	provider := &fakeStatusProvider{snapshots: []runtime.ReconciliationStatus{{LastOutcome: &success}, {LastOutcome: &failure}}}
	server := NewServer(runtime.New(), &fakeSetupService{completed: true}, provider, http.NotFoundHandler())
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))
	if provider.calls != 1 {
		t.Fatalf("Snapshot() calls = %d, want 1", provider.calls)
	}
	var body testStatusResponse
	decodeStrictJSON(t, recorder.Body.Bytes(), &body)
	if !body.Ready || body.Reconciliation.LastOutcome == nil || *body.Reconciliation.LastOutcome != "success" {
		t.Fatalf("response = %+v", body)
	}
}

func decodeStrictJSON(t *testing.T, raw []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatal(err)
	}
}

func TestReadyReasons(t *testing.T) {
	tests := []struct {
		name       string
		completed  bool
		setupErr   error
		configure  func(*runtime.ReconciliationTracker)
		wantStatus int
		wantReason string
	}{
		{"setup incomplete", false, nil, nil, http.StatusServiceUnavailable, "setup_incomplete"},
		{"pending", true, nil, nil, http.StatusServiceUnavailable, "reconciliation_pending"},
		{"failed", true, nil, func(tracker *runtime.ReconciliationTracker) { tracker.Start(); tracker.CompleteRuntimeBuildFailure() }, http.StatusServiceUnavailable, "reconciliation_failed"},
		{"ready", true, nil, func(tracker *runtime.ReconciliationTracker) { tracker.Start(); tracker.CompleteSuccess() }, http.StatusOK, "ready"},
		{"ready during refresh", true, nil, func(tracker *runtime.ReconciliationTracker) {
			tracker.Start()
			tracker.CompleteSuccess()
			tracker.Start()
		}, http.StatusOK, "ready"},
		{"status unavailable", true, errors.New("secret-key"), nil, http.StatusServiceUnavailable, "status_unavailable"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tracker := runtime.NewReconciliationTracker()
			if test.configure != nil {
				test.configure(tracker)
			}
			server := NewServer(runtime.New(), &fakeSetupService{completed: test.completed, err: test.setupErr}, tracker, http.NotFoundHandler())
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
			raw := recorder.Body.Bytes()
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type = %q", got)
			}
			if strings.Contains(string(raw), "secret-key") {
				t.Fatalf("secret exposed in response: %s", raw)
			}
			var body struct {
				Ready  bool   `json:"ready"`
				Reason string `json:"reason"`
			}
			decodeStrictJSON(t, raw, &body)
			if body.Reason != test.wantReason || body.Ready != (test.wantStatus == http.StatusOK) {
				t.Fatalf("body = %+v", body)
			}
		})
	}
}

func TestListServers(t *testing.T) {
	status := "running"

	discovery := &fakeDiscoveryService{
		servers: []models.MinecraftServer{
			{
				ID:           42,
				UUID:         "server-uuid",
				Identifier:   "abc12345",
				Name:         "Vanilla",
				NodeID:       3,
				EggID:        7,
				AllocationID: 11,
				Suspended:    false,
				Status:       &status,
			},
		},
	}

	server := newTestServer(
		discovery,
		&fakeRoutingService{},
		&fakeSetupService{},
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/servers",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf(
			"Content-Type = %q, want %q",
			got,
			"application/json",
		)
	}

	var response struct {
		Servers []models.MinecraftServer `json:"servers"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Servers) != 1 {
		t.Fatalf(
			"server count = %d, want 1",
			len(response.Servers),
		)
	}

	got := response.Servers[0]

	if got.ID != 42 {
		t.Errorf("server ID = %d, want 42", got.ID)
	}

	if got.Name != "Vanilla" {
		t.Errorf(
			"server name = %q, want %q",
			got.Name,
			"Vanilla",
		)
	}

	if got.AllocationID != 11 {
		t.Errorf(
			"allocation ID = %d, want 11",
			got.AllocationID,
		)
	}
}

func TestListServersReturnsInternalServerError(t *testing.T) {
	discovery := &fakeDiscoveryService{
		err: errors.New("Pelican unavailable"),
	}

	server := newTestServer(
		discovery,
		&fakeRoutingService{},
		&fakeSetupService{},
	)
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/servers",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expectedMessage = "failed to discover Minecraft servers"

	if response.Error != expectedMessage {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expectedMessage,
		)
	}
}

func TestUnknownRouteReturnsNotFound(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/unknown",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}
func TestListRoutes(t *testing.T) {
	routingService := &fakeRoutingService{
		routes: []routing.Route{
			{
				ServerID: "abc12345",
				Hostname: "vanilla.mc.example.com",
				Backend: routing.Backend{
					Host: "192.168.1.10",
					Port: 25565,
				},
			},
		},
	}

	server := newTestServer(
		&fakeDiscoveryService{},
		routingService,
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/routes",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if got := recorder.Header().Get("Content-Type"); got !=
		"application/json" {
		t.Errorf(
			"Content-Type = %q, want %q",
			got,
			"application/json",
		)
	}

	var response struct {
		Routes []routing.Route `json:"routes"`
	}

	if err := json.NewDecoder(recorder.Body).
		Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response.Routes) != 1 {
		t.Fatalf(
			"route count = %d, want 1",
			len(response.Routes),
		)
	}

	got := response.Routes[0]

	if got.ServerID != "abc12345" {
		t.Errorf(
			"server ID = %q, want %q",
			got.ServerID,
			"abc12345",
		)
	}

	if got.Hostname != "vanilla.mc.example.com" {
		t.Errorf(
			"hostname = %q, want %q",
			got.Hostname,
			"vanilla.mc.example.com",
		)
	}

	if got.Backend.Host != "192.168.1.10" {
		t.Errorf(
			"backend host = %q, want %q",
			got.Backend.Host,
			"192.168.1.10",
		)
	}

	if got.Backend.Port != 25565 {
		t.Errorf(
			"backend port = %d, want %d",
			got.Backend.Port,
			25565,
		)
	}
}

func TestListRoutesReturnsInternalServerError(t *testing.T) {
	routingService := &fakeRoutingService{
		err: errors.New("route generation failed"),
	}

	server := newTestServer(
		&fakeDiscoveryService{},
		routingService,
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/routes",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).
		Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expectedMessage = "failed to generate Minecraft routes"

	if response.Error != expectedMessage {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expectedMessage,
		)
	}
}
func TestGetSetupStatus(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{
			completed: true,
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/setup",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf(
			"Content-Type = %q, want %q",
			got,
			"application/json",
		)
	}

	var response struct {
		Completed bool `json:"completed"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.Completed {
		t.Errorf("completed = false, want true")
	}
}
func TestGetSetupStatusReturnsInternalServerError(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{
			err: errors.New("database unavailable"),
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/setup",
		nil,
	)
	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expected = "failed to get setup status"

	if response.Error != expected {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expected,
		)
	}
}
func TestConfigureSetup(t *testing.T) {
	setupService := &fakeSetupService{}

	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		setupService,
	)

	body := strings.NewReader(`{
		"pelican_url": " https://panel.example.com ",
		"pelican_api_key": " application-api-key ",
		"router_domain": " mc.example.com "
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		body,
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status code = %d, want %d; body = %q",
			recorder.Code,
			http.StatusNoContent,
			recorder.Body.String(),
		)
	}

	if recorder.Body.Len() != 0 {
		t.Errorf(
			"response body = %q, want empty body",
			recorder.Body.String(),
		)
	}

	if got := setupService.received.PelicanURL; got != "https://panel.example.com" {
		t.Errorf(
			"Pelican URL = %q, want %q",
			got,
			"https://panel.example.com",
		)
	}

	if got := setupService.received.PelicanAPIKey; got != "application-api-key" {
		t.Errorf(
			"Pelican API key = %q, want %q",
			got,
			"application-api-key",
		)
	}

	if got := setupService.received.RouterDomain; got != "mc.example.com" {
		t.Errorf(
			"router domain = %q, want %q",
			got,
			"mc.example.com",
		)
	}
}

func TestUpdateSettings(t *testing.T) {
	setupService := &fakeSetupService{}

	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		setupService,
	)

	body := strings.NewReader(`{
		"pelican_url": " https://panel.example.com ",
		"pelican_api_key": " application-api-key ",
		"router_domain": " mc.example.com "
	}`)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings",
		body,
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status code = %d, want %d; body = %q",
			recorder.Code,
			http.StatusNoContent,
			recorder.Body.String(),
		)
	}

	if recorder.Body.Len() != 0 {
		t.Errorf(
			"response body = %q, want empty body",
			recorder.Body.String(),
		)
	}

	if got := setupService.received.PelicanURL; got != "https://panel.example.com" {
		t.Errorf(
			"Pelican URL = %q, want %q",
			got,
			"https://panel.example.com",
		)
	}

	if got := setupService.received.PelicanAPIKey; got != "application-api-key" {
		t.Errorf(
			"Pelican API key = %q, want %q",
			got,
			"application-api-key",
		)
	}

	if got := setupService.received.RouterDomain; got != "mc.example.com" {
		t.Errorf(
			"router domain = %q, want %q",
			got,
			"mc.example.com",
		)
	}
}

func TestUpdateSettingsReturnsBadRequestForInvalidJSON(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings",
		strings.NewReader(`{"pelican_url":`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != "invalid request body" {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			"invalid request body",
		)
	}
}

func TestUpdateSettingsReturnsBadRequestForUnknownField(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_api_key": "key",
			"router_domain": "mc.example.com",
			"unexpected": true
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}

func TestUpdateSettingsReturnsBadRequestForMultipleJSONValues(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings",
		strings.NewReader(`{} {}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}

func TestUpdateSettingsReturnsInternalServerError(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{
			err: errors.New("settings update failed"),
		},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_api_key": "key",
			"router_domain": "mc.example.com"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != "failed to update settings" {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			"failed to update settings",
		)
	}
}

func TestConfigureSetupReturnsBadRequestForInvalidJSON(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		strings.NewReader(`{"pelican_url":`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != "invalid request body" {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			"invalid request body",
		)
	}
}
func TestConfigureSetupReturnsBadRequestForUnknownField(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_api_key": "key",
			"router_domain": "mc.example.com",
			"unexpected": true
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}
func TestConfigureSetupReturnsBadRequestForMultipleJSONValues(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		strings.NewReader(`{} {}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}
func TestConfigureSetupReturnsInternalServerError(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{
			err: errors.New("setup failed"),
		},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_api_key": "key",
			"router_domain": "mc.example.com"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != "failed to configure setup" {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			"failed to configure setup",
		)
	}
}

func TestUpdateSettingsReturnsBadRequestForMissingRouterDomain(
	t *testing.T,
) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{
			err: setup.ErrMissingRouterDomain,
		},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_api_key": "key",
			"router_domain": ""
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expected = "router domain is required"

	if response.Error != expected {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expected,
		)
	}
}

func TestConfigureSetupReturnsConflictWhenAlreadyConfigured(
	t *testing.T,
) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{
			err: setup.ErrAlreadyConfigured,
		},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_api_key": "key",
			"router_domain": "mc.example.com"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusConflict,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expected = "setup has already been completed"

	if response.Error != expected {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expected,
		)
	}
}

func TestListServersSetupIncomplete(t *testing.T) {
	server := newTestServer(
		nil,
		&fakeRoutingService{},
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/servers",
		nil,
	)

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusServiceUnavailable,
		)
	}
}
func TestListRoutesSetupIncomplete(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		nil,
		&fakeSetupService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/routes",
		nil,
	)

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusServiceUnavailable,
		)
	}
}
