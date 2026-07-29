package router

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/routepolicy"
	"github.com/zurco34/pelican-mc-router/pkg/models"
)

var ErrInvalidRoutePlan = errors.New("router: invalid route plan")

// Planner derives a complete desired route set without performing I/O.
type Planner struct {
	domain string
}

func NewPlanner(domain string) (*Planner, error) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return nil, errEmptyDomain
	}
	return &Planner{domain: domain}, nil
}

// Plan validates all desired routes before returning any of them. Policies are
// keyed by immutable Pelican server UUID; policies for undiscovered UUIDs are
// intentionally ignored here and remain durable state for later discovery.
func (p *Planner) Plan(
	servers []models.MinecraftServer,
	policies map[string]routepolicy.Policy,
) ([]Route, error) {
	routes := make([]Route, 0, len(servers))
	hostnames := make(map[string]struct{})

	for _, server := range servers {
		if server.Suspended {
			continue
		}
		if strings.TrimSpace(server.UUID) == "" {
			return nil, fmt.Errorf("route server identity: %w", ErrInvalidRoutePlan)
		}
		if strings.TrimSpace(server.BackendIP) == "" || server.BackendPort <= 0 {
			return nil, fmt.Errorf("route backend: %w", ErrInvalidRoutePlan)
		}

		policy, hasPolicy := policies[server.UUID]
		if hasPolicy && policy.ServerUUID != server.UUID {
			return nil, fmt.Errorf("route policy identity: %w", ErrInvalidRoutePlan)
		}
		if hasPolicy && policy.Excluded {
			if policy.PrimaryHostname != "" || len(policy.Aliases) != 0 {
				return nil, fmt.Errorf("excluded route policy: %w", ErrInvalidRoutePlan)
			}
			continue
		}

		primary := buildHostname(server, p.domain)
		if hasPolicy && policy.PrimaryHostname != "" {
			primary = policy.PrimaryHostname
		}
		if err := p.appendRoute(&routes, hostnames, server, primary); err != nil {
			return nil, err
		}
		if hasPolicy {
			for _, alias := range policy.Aliases {
				if err := p.appendRoute(&routes, hostnames, server, alias); err != nil {
					return nil, err
				}
			}
		}
	}

	return routes, nil
}

func (p *Planner) appendRoute(
	routes *[]Route,
	hostnames map[string]struct{},
	server models.MinecraftServer,
	hostname string,
) error {
	hostname = normalizeHostname(hostname)
	if !isValidHostname(hostname) {
		return fmt.Errorf("route hostname: %w", ErrInvalidRoutePlan)
	}
	if _, exists := hostnames[hostname]; exists {
		return fmt.Errorf("route hostname collision: %w", ErrInvalidRoutePlan)
	}
	hostnames[hostname] = struct{}{}
	*routes = append(*routes, Route{
		ServerID: server.UUID,
		Hostname: hostname,
		Backend:  Backend{Host: server.BackendIP, Port: server.BackendPort},
	})
	return nil
}

func normalizeHostname(hostname string) string {
	return strings.Trim(strings.ToLower(strings.TrimSpace(hostname)), ".")
}

func isValidHostname(hostname string) bool {
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for _, label := range strings.Split(hostname, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return net.ParseIP(hostname) == nil
}
