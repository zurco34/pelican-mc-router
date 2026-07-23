package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

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

type setupRequest struct {
	PelicanURL    string `json:"pelican_url"`
	PelicanAPIKey string `json:"pelican_api_key"`
	RouterDomain  string `json:"router_domain"`
}

type Server struct {
	runtime *runtime.Manager
	setup   SetupService
}

func NewServer(
	runtimeManager *runtime.Manager,
	setupService SetupService,
) *Server {
	return &Server{
		runtime: runtimeManager,
		setup:   setupService,
	}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/health", healthHandler)
	router.Get("/api/v1/servers", s.listServers)
	router.Get("/api/v1/routes", s.listRoutes)
	router.Get("/api/v1/setup", s.getSetupStatus)
	router.Post("/api/v1/setup", s.configureSetup)
	router.Put("/api/v1/settings", s.updateSettings)

	return router
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
