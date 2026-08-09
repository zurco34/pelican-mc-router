package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zurco34/pelican-mc-router/internal/actioncontrol"
	"github.com/zurco34/pelican-mc-router/internal/actionhistory"
	"github.com/zurco34/pelican-mc-router/internal/dashboard"
	"github.com/zurco34/pelican-mc-router/internal/operationalhistory"
	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/secretfile"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
	"github.com/zurco34/pelican-mc-router/pkg/buildinfo"
)

type SetupService interface {
	IsSetupComplete(context.Context) (bool, error)
	Setup(context.Context, settings.Settings) error
	Update(context.Context, settings.Settings) error
}

type ReconciliationStatusProvider interface {
	Snapshot() runtime.ReconciliationStatus
}

type DashboardService interface {
	Snapshot(context.Context) (dashboard.Snapshot, error)
}

type DashboardAuthorizer interface {
	Authorize(context.Context, *http.Request) error
	AuthorizeOperator(context.Context, *http.Request) error
}

type DashboardRefresher interface {
	Refresh(context.Context) error
}

type BootstrapAuthorizer interface {
	Authorize(*http.Request) error
}

type RoutePolicyStore interface {
	List(context.Context) ([]routepolicy.Policy, error)
	Create(context.Context, routepolicy.Policy) (routepolicy.Policy, error)
	Update(context.Context, routepolicy.Policy, int64) (routepolicy.Policy, error)
	Delete(context.Context, string, int64) error
}

type OperationalHistoryStore interface {
	List(context.Context, int) ([]operationalhistory.Event, error)
}

type ActionHistoryStore interface {
	List(context.Context, int) ([]actionhistory.Event, error)
}

type setupRequest struct {
	PelicanURL        string `json:"pelican_url"`
	PelicanSecretName string `json:"pelican_secret_name"`
	RouterDomain      string `json:"router_domain"`
}

type Server struct {
	runtime              *runtime.Manager
	setup                SetupService
	reconciliationStatus ReconciliationStatusProvider
	metrics              http.Handler
	build                buildinfo.Info
	dashboard            DashboardService
	dashboardAuth        DashboardAuthorizer
	dashboardRefresh     DashboardRefresher
	bootstrapAuth        BootstrapAuthorizer
	managementAuth       DashboardAuthorizer
	managementAuthSet    bool
	actionLimiter        *actioncontrol.Limiter
	routePolicies        RoutePolicyStore
	operationalHistory   OperationalHistoryStore
	actionHistoryReader  ActionHistoryStore
	actionHistory        interface {
		Append(context.Context, actionhistory.Event) error
	}
}

func (s *Server) WithActionHistory(store interface {
	Append(context.Context, actionhistory.Event) error
}) *Server {
	s.actionHistory = store
	return s
}

func (s *Server) recordAction(_ context.Context, action actionhistory.Action, outcome actionhistory.Outcome) {
	if s.actionHistory != nil {
		bounded, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := s.actionHistory.Append(bounded, actionhistory.Event{Action: action, Outcome: outcome}); err != nil {
			slog.Warn("record sensitive action history failed")
		}
	}
}

func (s *Server) WithActionLimiter(limiter *actioncontrol.Limiter) *Server {
	s.actionLimiter = limiter
	return s
}

func (s *Server) allowAction(w http.ResponseWriter, r *http.Request, action actioncontrol.Action) bool {
	if s.actionLimiter == nil || s.actionLimiter.Allow(action, time.Now()) {
		return true
	}
	writeJSONError(w, http.StatusTooManyRequests, "action temporarily unavailable")
	s.recordAction(r.Context(), actionhistory.ActionRateLimit, actionhistory.OutcomeDenied)
	return false
}

func NewServer(
	runtimeManager *runtime.Manager,
	setupService SetupService,
	reconciliationStatus ReconciliationStatusProvider,
	metrics http.Handler,
	build ...buildinfo.Info,
) *Server {
	info := buildinfo.Current()
	if len(build) > 0 {
		info = build[0]
	}

	return &Server{
		runtime:              runtimeManager,
		setup:                setupService,
		reconciliationStatus: reconciliationStatus,
		metrics:              metrics,
		build:                info,
	}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/health", healthHandler)
	router.Get("/metrics", s.metrics.ServeHTTP)
	router.Get("/ready", s.ready)
	router.Get("/api/v1/status", s.managementViewer(s.status))
	router.Get("/dashboard", s.managementViewer(s.dashboardPage))
	router.Post("/api/v1/dashboard/reconcile", s.managementOperator(s.reconcileDashboard))
	router.Get("/api/v1/servers", s.managementViewer(s.listServers))
	router.Get("/api/v1/routes", s.managementViewer(s.listRoutes))
	router.Get("/api/v1/routes/preview", s.managementViewer(s.previewRoutes))
	router.Get("/api/v1/route-policies", s.managementViewer(s.listRoutePolicies))
	router.Get("/api/v1/operational-history", s.managementViewer(s.listOperationalHistory))
	router.Get("/api/v1/action-history", s.managementViewer(s.listActionHistory))
	router.Post("/api/v1/route-policies", s.managementOperator(s.createRoutePolicy))
	router.Put("/api/v1/route-policies/{serverUUID}", s.managementOperator(s.updateRoutePolicy))
	router.Delete("/api/v1/route-policies/{serverUUID}", s.managementOperator(s.deleteRoutePolicy))
	router.Get("/api/v1/setup", s.bootstrapOnly(s.getSetupStatus))
	router.Post("/api/v1/setup", s.bootstrapOnly(s.configureSetup))
	router.Put("/api/v1/settings", s.managementOperator(s.updateSettings))

	return router
}

func (s *Server) WithRoutePolicies(store RoutePolicyStore) *Server { s.routePolicies = store; return s }

func (s *Server) WithOperationalHistory(store OperationalHistoryStore) *Server {
	s.operationalHistory = store
	return s
}

func (s *Server) WithActionHistoryReader(store ActionHistoryStore) *Server {
	s.actionHistoryReader = store
	return s
}

func (s *Server) listActionHistory(w http.ResponseWriter, r *http.Request) {
	if s.actionHistoryReader == nil {
		writeJSONError(w, http.StatusNotFound, "action history not found")
		return
	}
	limit := operationalhistory.MaxPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > operationalhistory.MaxPageSize {
			writeJSONError(w, http.StatusBadRequest, "invalid history limit")
			return
		}
		limit = value
	}
	events, err := s.actionHistoryReader.List(r.Context(), limit)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "action history unavailable")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Events []actionEventResponse `json:"events"`
	}{Events: actionEventsResponse(events)})
}

type operationalHistoryResponse struct {
	Events []operationalEventResponse `json:"events"`
}

func (s *Server) listOperationalHistory(w http.ResponseWriter, r *http.Request) {
	if s.operationalHistory == nil {
		writeJSONError(w, http.StatusNotFound, "operational history not found")
		return
	}
	limit := operationalhistory.MaxPageSize
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > operationalhistory.MaxPageSize {
			writeJSONError(w, http.StatusBadRequest, "invalid history limit")
			return
		}
		limit = value
	}
	events, err := s.operationalHistory.List(r.Context(), limit)
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "operational history unavailable")
		return
	}
	writeJSON(w, http.StatusOK, operationalHistoryResponse{Events: operationalEventsResponse(events)})
}

type routePolicyRequest struct {
	ServerUUID      string   `json:"server_uuid"`
	PrimaryHostname string   `json:"primary_hostname"`
	Aliases         []string `json:"aliases"`
	Excluded        bool     `json:"excluded"`
	Revision        int64    `json:"revision"`
}

func (s *Server) listRoutePolicies(w http.ResponseWriter, r *http.Request) {
	if s.routePolicies == nil {
		writeJSONError(w, http.StatusNotFound, "route policies not found")
		return
	}
	policies, err := s.routePolicies.List(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "route policies unavailable")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Policies []routePolicyResponse `json:"policies"`
	}{Policies: routePoliciesResponse(policies)})
}

func (s *Server) createRoutePolicy(w http.ResponseWriter, r *http.Request) {
	if s.routePolicies == nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusNotFound, "route policies not found")
		return
	}
	var request routePolicyRequest
	if !decodeJSON(w, r, &request) {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		return
	}
	policy, err := s.routePolicies.Create(r.Context(), routepolicy.Policy{ServerUUID: request.ServerUUID, PrimaryHostname: request.PrimaryHostname, Aliases: request.Aliases, Excluded: request.Excluded})
	if errors.Is(err, routepolicy.ErrInvalid) {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusBadRequest, "invalid route policy")
		return
	}
	if err != nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, outcomeForError(err))
		writeJSONError(w, http.StatusServiceUnavailable, "route policy unavailable")
		return
	}
	s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeSuccess)
	writeJSON(w, http.StatusCreated, routePolicyResponseFor(policy))
}

func (s *Server) updateRoutePolicy(w http.ResponseWriter, r *http.Request) {
	if s.routePolicies == nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusNotFound, "route policies not found")
		return
	}
	var request routePolicyRequest
	if !decodeJSON(w, r, &request) {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		return
	}
	policy, err := s.routePolicies.Update(r.Context(), routepolicy.Policy{ServerUUID: chi.URLParam(r, "serverUUID"), PrimaryHostname: request.PrimaryHostname, Aliases: request.Aliases, Excluded: request.Excluded}, request.Revision)
	if err != nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, outcomeForError(err))
	} else {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeSuccess)
	}
	s.writeRoutePolicyResult(w, policy, err)
}

func (s *Server) deleteRoutePolicy(w http.ResponseWriter, r *http.Request) {
	if s.routePolicies == nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusNotFound, "route policies not found")
		return
	}
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusBadRequest, "invalid route policy revision")
		return
	}
	err = s.routePolicies.Delete(r.Context(), chi.URLParam(r, "serverUUID"), revision)
	if errors.Is(err, routepolicy.ErrNotFound) {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusNotFound, "route policy not found")
		return
	}
	if errors.Is(err, routepolicy.ErrConflict) {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusConflict, "route policy conflict")
		return
	}
	if errors.Is(err, routepolicy.ErrInvalid) {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusBadRequest, "invalid route policy")
		return
	}
	if err != nil {
		s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, outcomeForError(err))
		writeJSONError(w, http.StatusServiceUnavailable, "route policy unavailable")
		return
	}
	s.recordAction(r.Context(), actionhistory.ActionRoutePolicy, actionhistory.OutcomeSuccess)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeRoutePolicyResult(w http.ResponseWriter, policy routepolicy.Policy, err error) {
	if errors.Is(err, routepolicy.ErrNotFound) {
		writeJSONError(w, http.StatusNotFound, "route policy not found")
		return
	}
	if errors.Is(err, routepolicy.ErrConflict) {
		writeJSONError(w, http.StatusConflict, "route policy conflict")
		return
	}
	if errors.Is(err, routepolicy.ErrInvalid) {
		writeJSONError(w, http.StatusBadRequest, "invalid route policy")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "route policy unavailable")
		return
	}
	writeJSON(w, http.StatusOK, routePolicyResponseFor(policy))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func (s *Server) WithBootstrapAuthorization(authorizer BootstrapAuthorizer) *Server {
	s.bootstrapAuth = authorizer
	return s
}

func (s *Server) bootstrapOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		completed, err := s.setup.IsSetupComplete(r.Context())
		if err != nil {
			s.recordAction(r.Context(), actionhistory.ActionBootstrap, actionhistory.OutcomeFailure)
			writeJSONError(w, http.StatusServiceUnavailable, "setup status unavailable")
			return
		}
		if completed {
			writeJSONError(w, http.StatusNotFound, "setup is unavailable")
			return
		}
		if s.bootstrapAuth == nil {
			s.recordAction(r.Context(), actionhistory.ActionBootstrap, actionhistory.OutcomeFailure)
			writeJSONError(w, http.StatusServiceUnavailable, "bootstrap authentication unavailable")
			return
		}
		if err := s.bootstrapAuth.Authorize(r); err != nil {
			s.recordAction(r.Context(), actionhistory.ActionBootstrap, actionhistory.OutcomeDenied)
			writeJSONError(w, http.StatusUnauthorized, "bootstrap authentication required")
			return
		}
		s.recordAction(r.Context(), actionhistory.ActionBootstrap, actionhistory.OutcomeSuccess)
		next(w, r)
	}
}

func (s *Server) WithDashboard(service DashboardService) *Server {
	s.dashboard = service
	return s
}

func (s *Server) WithDashboardAuthorization(authorizer DashboardAuthorizer) *Server {
	s.dashboardAuth = authorizer
	return s
}

// WithManagementAuthorization enables the v0.3 management-plane policy.
// A nil authorizer deliberately leaves management routes unavailable.
func (s *Server) WithManagementAuthorization(authorizer DashboardAuthorizer) *Server {
	s.managementAuth = authorizer
	s.managementAuthSet = true
	return s
}

func (s *Server) managementViewer(next http.HandlerFunc) http.HandlerFunc {
	return s.managementAuthorize(next, false)
}

func (s *Server) managementOperator(next http.HandlerFunc) http.HandlerFunc {
	return s.managementAuthorize(next, true)
}

func (s *Server) managementAuthorize(next http.HandlerFunc, operator bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.managementAuthSet {
			next(w, r)
			return
		}
		if s.managementAuth == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "management authentication unavailable")
			return
		}
		var err error
		if operator {
			err = s.managementAuth.AuthorizeOperator(r.Context(), r)
		} else {
			err = s.managementAuth.Authorize(r.Context(), r)
		}
		if err != nil {
			dashboardAuthorizationError(w, err)
			return
		}
		next(w, r)
	}
}

func (s *Server) WithDashboardActions(refresher DashboardRefresher) *Server {
	s.dashboardRefresh = refresher
	return s
}

type readiness struct {
	Ready  bool
	Reason string
}

type reconciliationStatusResponse struct {
	InProgress          bool                 `json:"in_progress"`
	LastOutcome         *string              `json:"last_outcome"`
	LastStartedAt       *string              `json:"last_started_at"`
	LastCompletedAt     *string              `json:"last_completed_at"`
	LastSuccessAt       *string              `json:"last_success_at"`
	LastDurationMS      int64                `json:"last_duration_ms"`
	ConsecutiveFailures int                  `json:"consecutive_failures"`
	LastError           *string              `json:"last_error"`
	RouteChanges        routeChangesResponse `json:"route_changes"`
}

type routeChangesResponse struct {
	Desired int  `json:"desired"`
	Created int  `json:"created"`
	Updated int  `json:"updated"`
	Deleted int  `json:"deleted"`
	Changed bool `json:"changed"`
}

type statusResponse struct {
	Build           buildinfo.Info               `json:"build"`
	SetupCompleted  bool                         `json:"setup_completed"`
	Ready           bool                         `json:"ready"`
	ReadinessReason string                       `json:"readiness_reason"`
	Reconciliation  reconciliationStatusResponse `json:"reconciliation"`
}

func readinessFor(
	completed bool,
	status runtime.ReconciliationStatus,
) readiness {
	if !completed {
		return readiness{Reason: "setup_incomplete"}
	}

	switch {
	case status.LastOutcome == nil,
		*status.LastOutcome == runtime.ReconciliationOutcomeNotConfigured:
		return readiness{Reason: "reconciliation_pending"}
	case *status.LastOutcome == runtime.ReconciliationOutcomeFailure:
		return readiness{Reason: "reconciliation_failed"}
	case *status.LastOutcome == runtime.ReconciliationOutcomeSuccess:
		return readiness{Ready: true, Reason: "ready"}
	default:
		return readiness{Reason: "reconciliation_pending"}
	}
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	completed, err := s.setup.IsSetupComplete(r.Context())
	if err != nil {
		slog.Error("get readiness failed")
		writeJSON(w, http.StatusServiceUnavailable, readinessResponse{Ready: false, Reason: "status_unavailable"})
		return
	}
	result := readinessFor(completed, s.reconciliationStatus.Snapshot())

	statusCode := http.StatusServiceUnavailable
	if result.Ready {
		statusCode = http.StatusOK
	}

	writeJSON(w, statusCode, readinessResponse{Ready: result.Ready, Reason: result.Reason})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	setupCompleted, err := s.setup.IsSetupComplete(r.Context())
	if err != nil {
		slog.Error("get application status failed")
		writeJSONError(w, http.StatusServiceUnavailable, "application status unavailable")
		return
	}

	reconciliationStatus := s.reconciliationStatus.Snapshot()
	result := readinessFor(setupCompleted, reconciliationStatus)

	writeJSON(w, http.StatusOK, statusResponse{
		Build:           s.build,
		SetupCompleted:  setupCompleted,
		Ready:           result.Ready,
		ReadinessReason: result.Reason,
		Reconciliation: reconciliationResponse(
			reconciliationStatus,
		),
	})
}

func reconciliationResponse(
	status runtime.ReconciliationStatus,
) reconciliationStatusResponse {
	return reconciliationStatusResponse{
		InProgress:          status.InProgress,
		LastOutcome:         outcomeString(status.LastOutcome),
		LastStartedAt:       timestampString(status.LastStartedAt),
		LastCompletedAt:     timestampString(status.LastCompletedAt),
		LastSuccessAt:       timestampString(status.LastSuccessAt),
		LastDurationMS:      status.LastDurationMS,
		ConsecutiveFailures: status.ConsecutiveFailures,
		LastError:           status.LastError,
		RouteChanges: routeChangesResponse{
			Desired: status.RouteChanges.Desired,
			Created: status.RouteChanges.Created,
			Updated: status.RouteChanges.Updated,
			Deleted: status.RouteChanges.Deleted,
			Changed: status.RouteChanges.Changed,
		},
	}
}

func outcomeString(value *runtime.ReconciliationOutcome) *string {
	if value == nil {
		return nil
	}

	result := string(*value)
	return &result
}

func timestampString(value *time.Time) *string {
	if value == nil {
		return nil
	}

	result := value.UTC().Format(time.RFC3339Nano)
	return &result
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("write health response failed")
	}
}

func (s *Server) listServers(
	w http.ResponseWriter,
	r *http.Request,
) {
	discovery := s.runtime.Discovery()
	if discovery == nil {
		writeSetupIncomplete(w)
		return
	}

	servers, err := discovery.Discover(r.Context())
	if err != nil {
		slog.Error("discover Minecraft servers failed")

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to discover Minecraft servers",
		)

		return
	}

	writeJSON(w, http.StatusOK, serversResponse{Servers: serversResponseFor(servers)})
}

func (s *Server) listRoutes(
	w http.ResponseWriter,
	r *http.Request,
) {
	routingService := s.runtime.Routing()
	if routingService == nil {
		writeSetupIncomplete(w)
		return
	}

	routes, err := routingService.Routes(r.Context())
	if err != nil {
		slog.Error("generate Minecraft routes failed")

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to generate Minecraft routes",
		)

		return
	}

	writeJSON(w, http.StatusOK, routesResponse{Routes: routesResponseFor(routes)})
}

type routePreviewResponse struct {
	Desired int             `json:"desired"`
	Routes  []routeResponse `json:"routes"`
}

// previewRoutes returns the cached inventory's planner output. The runtime
// inventory is seeded only by reconciliation, so this read path makes no
// Pelican or routing-backend request.
func (s *Server) previewRoutes(
	w http.ResponseWriter,
	r *http.Request,
) {
	routingService := s.runtime.Routing()
	if routingService == nil {
		writeSetupIncomplete(w)
		return
	}

	routes, err := routingService.Routes(r.Context())
	if err != nil {
		slog.Warn("generate cached route preview")
		writeJSONError(w, http.StatusServiceUnavailable, "route preview unavailable")
		return
	}

	writeJSON(w, http.StatusOK, routePreviewResponse{
		Desired: len(routes),
		Routes:  routesResponseFor(routes),
	})
}

func (s *Server) getSetupStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	completed, err := s.setup.IsSetupComplete(r.Context())
	if err != nil {
		slog.Error("get setup status failed")

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to get setup status",
		)

		return
	}

	writeJSON(w, http.StatusOK, setupStatusResponse{Completed: completed})
}

func (s *Server) configureSetup(
	w http.ResponseWriter,
	r *http.Request,
) {
	if !s.allowAction(w, r, actioncontrol.ActionSetup) {
		return
	}
	var request setupRequest

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		s.recordAction(r.Context(), actionhistory.ActionSetup, actionhistory.OutcomeFailure)
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}

	var trailing any

	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		s.recordAction(r.Context(), actionhistory.ActionSetup, actionhistory.OutcomeFailure)
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}
	if !secretfile.ValidName(strings.TrimSpace(request.PelicanSecretName)) {
		s.recordAction(r.Context(), actionhistory.ActionSetup, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusBadRequest, "invalid Pelican credential reference")
		return
	}

	setupSettings := settings.Settings{
		PelicanURL:        strings.TrimSpace(request.PelicanURL),
		PelicanSecretName: strings.TrimSpace(request.PelicanSecretName),
		RouterDomain:      strings.TrimSpace(request.RouterDomain),
	}

	if err := s.setup.Setup(r.Context(), setupSettings); err != nil {
		s.recordAction(r.Context(), actionhistory.ActionSetup, outcomeForError(err))
		if writeSetupError(w, err) {
			return
		}

		slog.Error("configure setup failed")

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to configure setup",
		)

		return
	}

	s.recordAction(r.Context(), actionhistory.ActionSetup, actionhistory.OutcomeSuccess)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateSettings(
	w http.ResponseWriter,
	r *http.Request,
) {
	if !s.allowAction(w, r, actioncontrol.ActionSettings) {
		return
	}
	var request setupRequest

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		s.recordAction(r.Context(), actionhistory.ActionSettings, actionhistory.OutcomeFailure)
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}

	var trailing any

	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		s.recordAction(r.Context(), actionhistory.ActionSettings, actionhistory.OutcomeFailure)
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}
	if !secretfile.ValidName(strings.TrimSpace(request.PelicanSecretName)) {
		s.recordAction(r.Context(), actionhistory.ActionSettings, actionhistory.OutcomeFailure)
		writeJSONError(w, http.StatusBadRequest, "invalid Pelican credential reference")
		return
	}

	updatedSettings := settings.Settings{
		PelicanURL:        strings.TrimSpace(request.PelicanURL),
		PelicanSecretName: strings.TrimSpace(request.PelicanSecretName),
		RouterDomain:      strings.TrimSpace(request.RouterDomain),
	}

	if err := s.setup.Update(r.Context(), updatedSettings); err != nil {
		s.recordAction(r.Context(), actionhistory.ActionSettings, outcomeForError(err))
		if writeSetupError(w, err) {
			return
		}

		slog.Error("update settings failed")

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to update settings",
		)

		return
	}
	s.recordAction(r.Context(), actionhistory.ActionSettings, actionhistory.OutcomeSuccess)
	w.WriteHeader(http.StatusNoContent)
}

func outcomeForError(err error) actionhistory.Outcome {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return actionhistory.OutcomeCanceled
	}
	return actionhistory.OutcomeFailure
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode HTTP response failed")
	}
}

func writeJSONError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func writeSetupIncomplete(w http.ResponseWriter) {
	writeJSONError(
		w,
		http.StatusServiceUnavailable,
		"setup has not been completed",
	)
}

func writeSetupError(
	w http.ResponseWriter,
	err error,
) bool {
	switch {
	case errors.Is(err, setup.ErrAlreadyConfigured):
		writeJSONError(
			w,
			http.StatusConflict,
			"setup has already been completed",
		)

	case errors.Is(err, setup.ErrMissingRouterDomain):
		writeJSONError(
			w,
			http.StatusBadRequest,
			"router domain is required",
		)

	case errors.Is(err, setup.ErrSetupNotActive):
		writeJSONError(w, http.StatusConflict, "setup has not been completed")

	default:
		return false
	}

	return true
}
