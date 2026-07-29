// Package dashboardauth verifies OIDC identities for the dashboard.
package dashboardauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/zurco34/pelican-mc-router/pkg/config"
)

var (
	ErrUnauthenticated   = errors.New("dashboard authentication required")
	ErrForbidden         = errors.New("dashboard authorization denied")
	ErrProviderDiscovery = errors.New("dashboard identity provider discovery failed")
)

type Authorizer interface {
	Authorize(context.Context, *http.Request) error
	AuthorizeOperator(context.Context, *http.Request) error
}

type verifier interface {
	Verify(context.Context, string) (identity, error)
}

type identity struct {
	subject string
	roles   []string
}

type oidcVerifier struct {
	verifier  *oidc.IDTokenVerifier
	roleClaim string
	client    *http.Client
}

type oidcAuthorizer struct {
	verifier     verifier
	roleClaim    string
	requiredRole string
	operatorRole string
}

// New creates an OIDC authorizer. Provider discovery happens during startup so
// a configured protected dashboard never starts with an unverified issuer.
func New(ctx context.Context, cfg config.DashboardAuthConfig) (Authorizer, error) {
	client := &http.Client{Timeout: cfg.DiscoveryTimeout}
	providerContext := oidc.ClientContext(ctx, client)
	provider, err := oidc.NewProvider(providerContext, cfg.IssuerURL)
	if err != nil {
		return nil, ErrProviderDiscovery
	}

	return &oidcAuthorizer{
		verifier: oidcVerifier{
			verifier:  provider.Verifier(&oidc.Config{ClientID: cfg.Audience}),
			roleClaim: cfg.RoleClaim,
			client:    client,
		},
		requiredRole: cfg.RequiredRole,
		operatorRole: cfg.OperatorRole,
	}, nil
}

func (a *oidcAuthorizer) Authorize(ctx context.Context, request *http.Request) error {
	return a.authorize(ctx, request, a.requiredRole, a.operatorRole)
}

func (a *oidcAuthorizer) AuthorizeOperator(ctx context.Context, request *http.Request) error {
	return a.authorize(ctx, request, a.operatorRole)
}

func (a *oidcAuthorizer) authorize(ctx context.Context, request *http.Request, requiredRoles ...string) error {
	token, err := bearerToken(request.Header.Get("Authorization"))
	if err != nil {
		return err
	}

	identity, err := a.verifier.Verify(ctx, token)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			return ErrForbidden
		}
		return ErrUnauthenticated
	}
	if strings.TrimSpace(identity.subject) == "" {
		return ErrUnauthenticated
	}
	if !containsAny(identity.roles, requiredRoles) {
		return ErrForbidden
	}

	return nil
}

func (v oidcVerifier) Verify(ctx context.Context, rawToken string) (identity, error) {
	ctx = oidc.ClientContext(ctx, v.client)
	token, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return identity{}, err
	}
	var claims map[string]json.RawMessage
	if err := token.Claims(&claims); err != nil {
		return identity{}, err
	}
	roles, ok := stringSliceClaim(claims[v.roleClaim])
	if !ok {
		return identity{}, ErrForbidden
	}
	return identity{subject: token.Subject, roles: roles}, nil
}

func bearerToken(value string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return "", ErrUnauthenticated
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if token == "" {
		return "", ErrUnauthenticated
	}
	return token, nil
}

func stringSliceClaim(value json.RawMessage) ([]string, bool) {
	var roles []string
	if len(value) == 0 || json.Unmarshal(value, &roles) != nil {
		return nil, false
	}
	return roles, true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsAny(values []string, targets []string) bool {
	for _, target := range targets {
		if contains(values, target) {
			return true
		}
	}
	return false
}
