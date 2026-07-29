package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zurco34/pelican-mc-router/pkg/models"
)

var (
	errNilDiscoveryService = errors.New(
		"discovery service must not be nil",
	)
	errEmptyDomain = errors.New(
		"router domain must not be empty",
	)
)

type DiscoveryService interface {
	Discover(context.Context) ([]models.MinecraftServer, error)
}

type Backend struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Route struct {
	ServerID string  `json:"server_id"`
	Hostname string  `json:"hostname"`
	Backend  Backend `json:"backend"`
}

type Service struct {
	discovery DiscoveryService
	planner   *Planner
}

func New(
	discovery DiscoveryService,
	domain string,
) (*Service, error) {
	if discovery == nil {
		return nil, errNilDiscoveryService
	}

	planner, err := NewPlanner(domain)
	if err != nil {
		return nil, err
	}

	return &Service{
		discovery: discovery,
		planner:   planner,
	}, nil
}

func (s *Service) Routes(
	ctx context.Context,
) ([]Route, error) {
	servers, err := s.discovery.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover Minecraft servers: %w", err)
	}

	routes, err := s.planner.Plan(servers, nil)
	if err != nil {
		return nil, fmt.Errorf("plan routes: %w", err)
	}
	return routes, nil
}

func buildHostname(
	server models.MinecraftServer,
	domain string,
) string {
	label := normalizeHostnameLabel(server.Name)
	if label == "" {
		label = normalizeHostnameLabel(server.Identifier)
	}

	if label == "" {
		return ""
	}

	return label + "." + domain
}

func normalizeDomain(domain string) string {
	return strings.Trim(
		strings.ToLower(strings.TrimSpace(domain)),
		".",
	)
}

func normalizeHostnameLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))

	var builder strings.Builder
	builder.Grow(len(value))

	lastWasHyphen := false

	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
			lastWasHyphen = false

		case character >= '0' && character <= '9':
			builder.WriteRune(character)
			lastWasHyphen = false

		case character == '-' ||
			character == '_' ||
			character == ' ':
			if builder.Len() > 0 && !lastWasHyphen {
				builder.WriteByte('-')
				lastWasHyphen = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}
