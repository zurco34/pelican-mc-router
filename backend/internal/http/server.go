package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	routing "github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type DiscoveryService interface {
	Discover(context.Context) ([]models.MinecraftServer, error)
}

type RoutingService interface {
	Routes(context.Context) ([]routing.Route, error)
}
type SetupStatusService interface {
	IsSetupComplete(context.Context) (bool, error)
}
type Server struct {
	discovery DiscoveryService
	routing   RoutingService
	setup     SetupStatusService
}

func NewServer(
	discovery DiscoveryService,
	routing RoutingService,
	setup SetupStatusService,
) *Server {
	return &Server{
		discovery: discovery,
		routing:   routing,
		setup:     setup,
	}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/health", healthHandler)
	router.Get("/api/v1/servers", s.listServers)
	router.Get("/api/v1/routes", s.listRoutes)
	router.Get("/api/v1/setup", s.getSetupStatus)
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
	servers, err := s.discovery.Discover(r.Context())
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
	routes, err := s.routing.Routes(r.Context())
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

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("encode HTTP response", "error", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
