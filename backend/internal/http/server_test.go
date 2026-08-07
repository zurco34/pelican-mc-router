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

	"github.com/zurco34/pelican-mc-router/internal/actionhistory"
	"github.com/zurco34/pelican-mc-router/internal/dashboard"
	"github.com/zurco34/pelican-mc-router/internal/dashboardauth"
	"github.com/zurco34/pelican-mc-router/internal/observability"
	"github.com/zurco34/pelican-mc-router/internal/operationalhistory"
	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
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

type fakeOperationalHistoryStore struct {
	events []operationalhistory.Event
	err    error
	limit  int
}

type fakeActionHistoryWriter struct {
	events []actionhistory.Event
	err    error
}

func (f *fakeActionHistoryWriter) Append(_ context.Context, event actionhistory.Event) error {
	f.events = append(f.events, event)
	return f.err
}

type fakeRoutePolicyStore struct {
	policy routepolicy.Policy
	err    error
}

func (f *fakeRoutePolicyStore) List(context.Context) ([]routepolicy.Policy, error) {
	return nil, f.err
}

func (f *fakeRoutePolicyStore) Create(_ context.Context, policy routepolicy.Policy) (routepolicy.Policy, error) {
	if f.err != nil {
		return routepolicy.Policy{}, f.err
	}
	f.policy = policy
	return policy, nil
}

func (f *fakeRoutePolicyStore) Update(_ context.Context, policy routepolicy.Policy, _ int64) (routepolicy.Policy, error) {
	if f.err != nil {
		return routepolicy.Policy{}, f.err
	}
	f.policy = policy
	return policy, nil
}

func (f *fakeRoutePolicyStore) Delete(context.Context, string, int64) error { return f.err }

func (f *fakeOperationalHistoryStore) List(_ context.Context, limit int) ([]operationalhistory.Event, error) {
	f.limit = limit
	return f.events, f.err
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

type fakeBootstrapAuthorizer struct {
	err   error
	calls int
}

func (f *fakeBootstrapAuthorizer) Authorize(*http.Request) error {
	f.calls++
	return f.err
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
	).WithBootstrapAuthorization(&fakeBootstrapAuthorizer{})
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

func TestDashboardManualReconcileUnavailableUsesJSONError(t *testing.T) {
	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler())
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/dashboard/reconcile", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content type = %q, want application/json", got)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil || body.Error == "" {
		t.Fatalf("response = %q, decode error = %v", recorder.Body.String(), err)
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
		{"setup error", &fakeSetupService{err: errors.New("https://secret.example/api-key")}, runtime.ReconciliationStatus{}, http.StatusServiceUnavailable, false, ""},
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
				if body.Error != "application status unavailable" {
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

func TestPreviewRoutes(t *testing.T) {
	server := newTestServer(&fakeDiscoveryService{}, &fakeRoutingService{routes: []routing.Route{{
		ServerID: "server-uuid", Hostname: "preview.mc.example.com",
		Backend: routing.Backend{Host: "192.168.1.10", Port: 25565},
	}}}, &fakeSetupService{})
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/routes/preview", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Desired int             `json:"desired"`
		Routes  []routing.Route `json:"routes"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if response.Desired != 1 || len(response.Routes) != 1 {
		t.Fatalf("preview = %#v", response)
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
		&fakeSetupService{},
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

	if response.Completed {
		t.Errorf("completed = true, want false")
	}
}

func TestBootstrapOnlySetupRoutes(t *testing.T) {
	tests := []struct {
		name       string
		completed  bool
		authErr    error
		wantStatus int
		wantCalls  int
	}{
		{name: "authorized uninitialized", wantStatus: http.StatusOK, wantCalls: 1},
		{name: "missing or invalid token", authErr: errors.New("bootstrap-token-value"), wantStatus: http.StatusUnauthorized, wantCalls: 1},
		{name: "completed setup is unavailable", completed: true, wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authorizer := &fakeBootstrapAuthorizer{err: test.authErr}
			server := newTestServer(&fakeDiscoveryService{}, &fakeRoutingService{}, &fakeSetupService{completed: test.completed}).WithBootstrapAuthorization(authorizer)
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/setup", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if authorizer.calls != test.wantCalls {
				t.Fatalf("authorization calls = %d, want %d", authorizer.calls, test.wantCalls)
			}
			if strings.Contains(recorder.Body.String(), "bootstrap-token-value") {
				t.Fatal("bootstrap error exposed in response")
			}
		})
	}
}

func TestBootstrapOnlySetupRoutesRemainClosedWithoutAuthorizer(t *testing.T) {
	tests := []struct {
		name       string
		completed  bool
		setupErr   error
		wantStatus int
	}{
		{name: "completed after restart", completed: true, wantStatus: http.StatusNotFound},
		{name: "incomplete fails closed", wantStatus: http.StatusServiceUnavailable},
		{name: "setup lookup failure", setupErr: errors.New("database unavailable"), wantStatus: http.StatusServiceUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtimeManager := runtime.New()
			runtimeManager.Set(&fakeDiscoveryService{}, &fakeRoutingService{})
			server := NewServer(runtimeManager, &fakeSetupService{completed: test.completed, err: test.setupErr}, runtime.NewReconciliationTracker(), http.NotFoundHandler())
			recorder := httptest.NewRecorder()
			server.Router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/setup", nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
			if strings.Contains(recorder.Body.String(), "database unavailable") {
				t.Fatal("setup error exposed in response")
			}
		})
	}
}
func TestGetSetupStatusReturnsServiceUnavailableWhenSetupLookupFails(t *testing.T) {
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

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusServiceUnavailable,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expected = "setup status unavailable"

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
		"pelican_secret_name": " pelican-token ",
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

	if got := setupService.received.PelicanSecretName; got != "pelican-token" {
		t.Errorf(
			"Pelican secret name = %q, want %q",
			got,
			"pelican-token",
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
		"pelican_secret_name": " pelican-token ",
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

	if got := setupService.received.PelicanSecretName; got != "pelican-token" {
		t.Errorf(
			"Pelican secret name = %q, want %q",
			got,
			"pelican-token",
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
			"pelican_secret_name": "pelican-token",
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
			"pelican_secret_name": "pelican-token",
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
			"pelican_secret_name": "pelican-token",
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
func TestConfigureSetupFailsClosedWhenSetupLookupFails(t *testing.T) {
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
			"pelican_secret_name": "pelican-token",
			"router_domain": "mc.example.com"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusServiceUnavailable,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != "setup status unavailable" {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			"setup status unavailable",
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
			"pelican_secret_name": "pelican-token",
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

func TestConfigureSetupReturnsNotFoundWhenAlreadyConfigured(
	t *testing.T,
) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
		&fakeSetupService{completed: true},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/setup",
		strings.NewReader(`{
			"pelican_url": "https://panel.example.com",
			"pelican_secret_name": "pelican-token",
			"router_domain": "mc.example.com"
		}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()

	server.Router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"status code = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
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

func TestOperationalHistoryEndpointIsBoundedAndSafe(t *testing.T) {
	store := &fakeOperationalHistoryStore{events: []operationalhistory.Event{{Kind: operationalhistory.KindReconciliation, Outcome: operationalhistory.OutcomeSuccess, Desired: 2}}}
	server := NewServer(runtime.New(), &fakeSetupService{}, runtime.NewReconciliationTracker(), http.NotFoundHandler()).WithOperationalHistory(store)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/operational-history?limit=1", nil)
	recorder := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if store.limit != 1 {
		t.Fatalf("history limit = %d, want 1", store.limit)
	}
	if strings.Contains(recorder.Body.String(), "error") {
		t.Fatalf("response must not contain error data: %s", recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/operational-history?limit=101", nil)
	recorder = httptest.NewRecorder()
	server.Router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestRoutePolicyActionsAreRecordedWithFixedOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		handler func(*Server, http.ResponseWriter, *http.Request)
		request *http.Request
		store   *fakeRoutePolicyStore
		want    actionhistory.Outcome
	}{
		{
			name: "create success",
			handler: func(server *Server, w http.ResponseWriter, r *http.Request) {
				server.createRoutePolicy(w, r)
			},
			request: httptest.NewRequest(http.MethodPost, "/api/v1/route-policies", strings.NewReader(`{"server_uuid":"server","primary_hostname":"play.example.test"}`)),
			store:   &fakeRoutePolicyStore{},
			want:    actionhistory.OutcomeSuccess,
		},
		{
			name: "create failure",
			handler: func(server *Server, w http.ResponseWriter, r *http.Request) {
				server.createRoutePolicy(w, r)
			},
			request: httptest.NewRequest(http.MethodPost, "/api/v1/route-policies", strings.NewReader(`{"server_uuid":"server"}`)),
			store:   &fakeRoutePolicyStore{err: errors.New("store unavailable")},
			want:    actionhistory.OutcomeFailure,
		},
		{
			name: "update cancellation",
			handler: func(server *Server, w http.ResponseWriter, r *http.Request) {
				server.updateRoutePolicy(w, r)
			},
			request: httptest.NewRequest(http.MethodPut, "/api/v1/route-policies/server", strings.NewReader(`{"revision":1}`)),
			store:   &fakeRoutePolicyStore{err: context.Canceled},
			want:    actionhistory.OutcomeCanceled,
		},
		{
			name: "delete success",
			handler: func(server *Server, w http.ResponseWriter, r *http.Request) {
				server.deleteRoutePolicy(w, r)
			},
			request: httptest.NewRequest(http.MethodDelete, "/api/v1/route-policies/server?revision=1", nil),
			store:   &fakeRoutePolicyStore{},
			want:    actionhistory.OutcomeSuccess,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writer := &fakeActionHistoryWriter{}
			server := newTestServer(&fakeDiscoveryService{}, &fakeRoutingService{}, &fakeSetupService{}).
				WithRoutePolicies(test.store).
				WithActionHistory(writer)
			test.handler(server, httptest.NewRecorder(), test.request)
			if len(writer.events) != 1 {
				t.Fatalf("recorded events = %#v, want one", writer.events)
			}
			event := writer.events[0]
			if event.Action != actionhistory.ActionRoutePolicy || event.Outcome != test.want {
				t.Fatalf("recorded event = %#v, want route policy/%s", event, test.want)
			}
		})
	}
}

func TestSetupAndSettingsFailuresAreRecordedWithoutChangingResponse(t *testing.T) {
	for _, path := range []string{"/api/v1/setup", "/api/v1/settings"} {
		t.Run(path, func(t *testing.T) {
			writer := &fakeActionHistoryWriter{err: errors.New("history unavailable")}
			server := newTestServer(&fakeDiscoveryService{}, &fakeRoutingService{}, &fakeSetupService{}).WithActionHistory(writer)
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"unknown":true}`))
			recorder := httptest.NewRecorder()
			if path == "/api/v1/setup" {
				server.configureSetup(recorder, request)
			} else {
				server.updateSettings(recorder, request)
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if len(writer.events) != 1 {
				t.Fatalf("recorded events = %#v, want one", writer.events)
			}
			if writer.events[0].Outcome != actionhistory.OutcomeFailure {
				t.Fatalf("outcome = %s, want failure", writer.events[0].Outcome)
			}
		})
	}
}
