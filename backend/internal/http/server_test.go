package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	routing "github.com/zurco34/pelican-mc-router/internal/router"
	"github.com/zurco34/pelican-mc-router/internal/runtime"
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

func newTestServer(
	discovery runtime.DiscoveryService,
	routingService runtime.RoutingService,
) *Server {
	manager := runtime.New()
	manager.Set(discovery, routingService)

	return NewServer(manager)
}

func TestHealthHandler(t *testing.T) {
	server := newTestServer(
		&fakeDiscoveryService{},
		&fakeRoutingService{},
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

	server := newTestServer(discovery, &fakeRoutingService{})
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

	server := newTestServer(discovery, &fakeRoutingService{})

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
func TestListServersReturnsServiceUnavailableWhenRuntimeIsEmpty(t *testing.T) {
	manager := runtime.New()
	server := NewServer(manager)

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

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expectedMessage = "runtime services are not available"

	if response.Error != expectedMessage {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expectedMessage,
		)
	}
}

func TestListRoutesReturnsServiceUnavailableWhenRuntimeIsEmpty(t *testing.T) {
	manager := runtime.New()
	server := NewServer(manager)

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

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	const expectedMessage = "runtime services are not available"

	if response.Error != expectedMessage {
		t.Errorf(
			"error = %q, want %q",
			response.Error,
			expectedMessage,
		)
	}
}
