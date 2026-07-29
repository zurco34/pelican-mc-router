package config

import "time"

// DashboardAuthConfig configures optional OIDC authentication for /dashboard.
// The reverse proxy is responsible for forwarding a bearer token from the SSO.
type DashboardAuthConfig struct {
	Enabled          bool          `mapstructure:"enabled"`
	IssuerURL        string        `mapstructure:"issuer_url"`
	Audience         string        `mapstructure:"audience"`
	RoleClaim        string        `mapstructure:"role_claim"`
	RequiredRole     string        `mapstructure:"required_role"`
	DiscoveryTimeout time.Duration `mapstructure:"discovery_timeout"`
}
