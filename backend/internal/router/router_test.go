package router

import (
	"context"
	"errors"
	"testing"

	"github.com/zurco34/pelican-mc-router/pkg/models"
)

type fakeDiscoveryService struct {
	servers []models.MinecraftServer
	err     error
	calls   int
}

func (f *fakeDiscoveryService) Discover(
	context.Context,
) ([]models.MinecraftServer, error) {
	f.calls++

	return f.servers, f.err
}

func TestNewRejectsNilDiscoveryService(t *testing.T) {
	_, err := New(nil, "mc.example.com")
	if err == nil {
		t.Fatal("New() error = nil, want error")
	}

	if !errors.Is(err, errNilDiscoveryService) {
		t.Errorf(
			"New() error = %v, want %v",
			err,
			errNilDiscoveryService,
		)
	}
}

func TestNewRejectsEmptyDomain(t *testing.T) {
	discovery := &fakeDiscoveryService{}

	_, err := New(discovery, "   ")
	if err == nil {
		t.Fatal("New() error = nil, want error")
	}

	if !errors.Is(err, errEmptyDomain) {
		t.Errorf(
			"New() error = %v, want %v",
			err,
			errEmptyDomain,
		)
	}
}

func TestServiceRoutesGeneratesRoutes(t *testing.T) {
	discovery := &fakeDiscoveryService{
		servers: []models.MinecraftServer{
			{
				Identifier:  "vanilla",
				Name:        "Vanilla Survival",
				BackendIP:   "192.168.1.10",
				BackendPort: 25565,
			},
			{
				Identifier:  "atm10",
				Name:        "All The Mods 10",
				BackendIP:   "192.168.1.11",
				BackendPort: 25566,
			},
		},
	}

	service, err := New(discovery, ".MC.Example.COM.")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	routes, err := service.Routes(context.Background())
	if err != nil {
		t.Fatalf("Routes() error = %v", err)
	}

	if discovery.calls != 1 {
		t.Errorf(
			"Discover() calls = %d, want 1",
			discovery.calls,
		)
	}

	if len(routes) != 2 {
		t.Fatalf(
			"Routes() returned %d routes, want 2",
			len(routes),
		)
	}

	if routes[0].ServerID != "vanilla" {
		t.Errorf(
			"first ServerID = %q, want %q",
			routes[0].ServerID,
			"vanilla",
		)
	}

	if routes[0].Hostname !=
		"vanilla-survival.mc.example.com" {
		t.Errorf(
			"first Hostname = %q",
			routes[0].Hostname,
		)
	}

	if routes[0].Backend.Host != "192.168.1.10" {
		t.Errorf(
			"first backend host = %q",
			routes[0].Backend.Host,
		)
	}

	if routes[0].Backend.Port != 25565 {
		t.Errorf(
			"first backend port = %d",
			routes[0].Backend.Port,
		)
	}

	if routes[1].Hostname !=
		"all-the-mods-10.mc.example.com" {
		t.Errorf(
			"second Hostname = %q",
			routes[1].Hostname,
		)
	}
}

func TestServiceRoutesSkipsSuspendedServers(t *testing.T) {
	discovery := &fakeDiscoveryService{
		servers: []models.MinecraftServer{
			{
				Identifier:  "active",
				Name:        "Active",
				BackendIP:   "192.168.1.10",
				BackendPort: 25565,
			},
			{
				Identifier:  "suspended",
				Name:        "Suspended",
				BackendIP:   "192.168.1.11",
				BackendPort: 25566,
				Suspended:   true,
			},
		},
	}

	service, err := New(discovery, "mc.example.com")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	routes, err := service.Routes(context.Background())
	if err != nil {
		t.Fatalf("Routes() error = %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf(
			"Routes() returned %d routes, want 1",
			len(routes),
		)
	}

	if routes[0].ServerID != "active" {
		t.Errorf(
			"ServerID = %q, want %q",
			routes[0].ServerID,
			"active",
		)
	}
}

func TestServiceRoutesUsesIdentifierAsHostnameFallback(
	t *testing.T,
) {
	discovery := &fakeDiscoveryService{
		servers: []models.MinecraftServer{
			{
				Identifier:  "server-123",
				Name:        "!!!",
				BackendIP:   "192.168.1.10",
				BackendPort: 25565,
			},
		},
	}

	service, err := New(discovery, "mc.example.com")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	routes, err := service.Routes(context.Background())
	if err != nil {
		t.Fatalf("Routes() error = %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf(
			"Routes() returned %d routes, want 1",
			len(routes),
		)
	}

	if routes[0].Hostname !=
		"server-123.mc.example.com" {
		t.Errorf(
			"Hostname = %q",
			routes[0].Hostname,
		)
	}
}

func TestServiceRoutesReturnsDiscoveryError(t *testing.T) {
	expectedErr := errors.New("discovery failed")

	discovery := &fakeDiscoveryService{
		err: expectedErr,
	}

	service, err := New(discovery, "mc.example.com")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = service.Routes(context.Background())
	if err == nil {
		t.Fatal("Routes() error = nil, want error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf(
			"Routes() error = %v, want wrapped %v",
			err,
			expectedErr,
		)
	}
}

func TestServiceRoutesRejectsEmptyBackendIP(t *testing.T) {
	discovery := &fakeDiscoveryService{
		servers: []models.MinecraftServer{
			{
				Identifier:  "vanilla",
				Name:        "Vanilla",
				BackendPort: 25565,
			},
		},
	}

	service, err := New(discovery, "mc.example.com")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = service.Routes(context.Background())
	if err == nil {
		t.Fatal("Routes() error = nil, want error")
	}
}

func TestServiceRoutesRejectsInvalidBackendPort(t *testing.T) {
	discovery := &fakeDiscoveryService{
		servers: []models.MinecraftServer{
			{
				Identifier: "vanilla",
				Name:       "Vanilla",
				BackendIP:  "192.168.1.10",
			},
		},
	}

	service, err := New(discovery, "mc.example.com")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = service.Routes(context.Background())
	if err == nil {
		t.Fatal("Routes() error = nil, want error")
	}
}
