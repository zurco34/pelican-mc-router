package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
	"github.com/zurco34/pelican-mc-router/internal/settings"
	"github.com/zurco34/pelican-mc-router/internal/setup"
)

type SetupService interface {
	IsSetupComplete(context.Context) (bool, error)
	Setup(context.Context, settings.Settings) error
	Update(context.Context, settings.Settings) error
}

type ReconciliationStatusProvider interface {
	Snapshot() runtime.ReconciliationStatus
}

type setupRequest struct {
	PelicanURL    string `json:"pelican_url"`
	PelicanAPIKey string `json:"pelican_api_key"`
	RouterDomain  string `json:"router_domain"`
}

type Server struct {
	runtime              *runtime.Manager
	setup                SetupService
	reconciliationStatus ReconciliationStatusProvider
	metrics              http.Handler
}

func NewServer(
	runtimeManager *runtime.Manager,
	setupService SetupService,
	reconciliationStatus ReconciliationStatusProvider,
	metrics http.Handler,
) *Server {
	return &Server{
		runtime:              runtimeManager,
		setup:                setupService,
		reconciliationStatus: reconciliationStatus,
		metrics:              metrics,
	}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/health", healthHandler)
	router.Get("/metrics", s.metrics.ServeHTTP)
	router.Get("/ready", s.ready)
	router.Get("/api/v1/status", s.status)
	router.Get("/api/v1/servers", s.listServers)
	router.Get("/api/v1/routes", s.listRoutes)
	router.Get("/api/v1/setup", s.getSetupStatus)
	router.Post("/api/v1/setup", s.configureSetup)
	router.Put("/api/v1/settings", s.updateSettings)

	return router
}

type readiness struct {
	Ready  bool
	Reason string
}

type reconciliationStatusResponse struct {
	InProgress          bool    `json:"in_progress"`
	LastOutcome         *string `json:"last_outcome"`
	LastStartedAt       *string `json:"last_started_at"`
	LastCompletedAt     *string `json:"last_completed_at"`
	LastSuccessAt       *string `json:"last_success_at"`
	LastDurationMS      int64   `json:"last_duration_ms"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
	LastError           *string `json:"last_error"`
}

type statusResponse struct {
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
		slog.Error("get readiness", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ready":  false,
			"reason": "status_unavailable",
		})
		return
	}
	result := readinessFor(completed, s.reconciliationStatus.Snapshot())

	statusCode := http.StatusServiceUnavailable
	if result.Ready {
		statusCode = http.StatusOK
	}

	writeJSON(w, statusCode, map[string]any{
		"ready":  result.Ready,
		"reason": result.Reason,
	})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	setupCompleted, err := s.setup.IsSetupComplete(r.Context())
	if err != nil {
		slog.Error("get application status", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "failed to get application status")
		return
	}

	reconciliationStatus := s.reconciliationStatus.Snapshot()
	result := readinessFor(setupCompleted, reconciliationStatus)

	writeJSON(w, http.StatusOK, statusResponse{
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
		slog.Error("write health response", "error", err)
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
		slog.Error("discover Minecraft servers", "error", err)

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to discover Minecraft servers",
		)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"servers": servers,
	})
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
		slog.Error("generate Minecraft routes", "error", err)

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to generate Minecraft routes",
		)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"routes": routes,
	})
}

func (s *Server) getSetupStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	completed, err := s.setup.IsSetupComplete(r.Context())
	if err != nil {
		slog.Error("get setup status", "error", err)

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to get setup status",
		)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"completed": completed,
	})
}

func (s *Server) configureSetup(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request setupRequest

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}

	var trailing any

	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}

	setupSettings := settings.Settings{
		PelicanURL:    strings.TrimSpace(request.PelicanURL),
		PelicanAPIKey: strings.TrimSpace(request.PelicanAPIKey),
		RouterDomain:  strings.TrimSpace(request.RouterDomain),
	}

	if err := s.setup.Setup(r.Context(), setupSettings); err != nil {
		if writeSetupError(w, err) {
			return
		}

		slog.Error("configure setup", "error", err)

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to configure setup",
		)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) updateSettings(
	w http.ResponseWriter,
	r *http.Request,
) {
	var request setupRequest

	decoder := json.NewDecoder(
		http.MaxBytesReader(w, r.Body, 1<<20),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}

	var trailing any

	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSONError(
			w,
			http.StatusBadRequest,
			"invalid request body",
		)

		return
	}

	updatedSettings := settings.Settings{
		PelicanURL:    strings.TrimSpace(request.PelicanURL),
		PelicanAPIKey: strings.TrimSpace(request.PelicanAPIKey),
		RouterDomain:  strings.TrimSpace(request.RouterDomain),
	}

	if err := s.setup.Update(r.Context(), updatedSettings); err != nil {
		if writeSetupError(w, err) {
			return
		}

		slog.Error("update settings", "error", err)

		writeJSONError(
			w,
			http.StatusInternalServerError,
			"failed to update settings",
		)

		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode HTTP response", "error", err)
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

	default:
		return false
	}

	return true
}
