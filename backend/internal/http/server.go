package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
)

type Server struct {
	runtime *runtime.Manager
}

func NewServer(
	runtimeManager *runtime.Manager,
) *Server {
	return &Server{
		runtime: runtimeManager,
	}
}

func (s *Server) Router() http.Handler {
	router := chi.NewRouter()

	router.Get("/health", healthHandler)
	router.Get("/api/v1/servers", s.listServers)
	router.Get("/api/v1/routes", s.listRoutes)

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
		writeJSONError(
			w,
			http.StatusServiceUnavailable,
			"runtime services are not available",
		)
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
		writeJSONError(
			w,
			http.StatusServiceUnavailable,
			"runtime services are not available",
		)
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
