package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type DiscoveryService interface {
	Discover(context.Context) ([]models.MinecraftServer, error)
}

type Server struct {
	discovery DiscoveryService
}

func NewServer(discovery DiscoveryService) *Server {
	return &Server{
		discovery: discovery,
	}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/health", healthHandler)
	router.Get("/api/v1/servers", s.listServers)

	return router
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("write health response", "error", err)
	}
}

func (s *Server) listServers(w http.ResponseWriter, r *http.Request) {
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

var errNilDiscoveryService = errors.New(
	"HTTP server requires a discovery service",
)

type unavailableDiscoveryService struct{}

func (*unavailableDiscoveryService) Discover(
	context.Context,
) ([]models.MinecraftServer, error) {
	return nil, errors.New("discovery service is not configured")
}
