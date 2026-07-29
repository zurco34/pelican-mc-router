package router

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
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
type PolicySource interface {
	List(context.Context) ([]routepolicy.Policy, error)
}
type Option func(*Service)

func WithPolicySource(source PolicySource) Option { return func(s *Service) { s.policies = source } }

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
	policies  PolicySource
}

func New(
	discovery DiscoveryService,
	domain string,
	options ...Option,
) (*Service, error) {
	if discovery == nil {
		return nil, errNilDiscoveryService
	}

	planner, err := NewPlanner(domain)
	if err != nil {
		return nil, err
	}

	service := &Service{
		discovery: discovery,
		planner:   planner,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

func (s *Service) Routes(
	ctx context.Context,
) ([]Route, error) {
	servers, err := s.discovery.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover Minecraft servers: %w", err)
	}

	policies := map[string]routepolicy.Policy{}
	if s.policies != nil {
		stored, err := s.policies.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("load route policies: %w", err)
		}
		for _, policy := range stored {
			policies[policy.ServerUUID] = policy
		}
	}
	routes, err := s.planner.Plan(servers, policies)
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
