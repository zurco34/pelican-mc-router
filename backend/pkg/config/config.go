package config

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Pelican       PelicanConfig       `mapstructure:"pelican"`
	Discovery     DiscoveryConfig     `mapstructure:"discovery"`
	Router        RouterConfig        `mapstructure:"router"`
	MCRouter      MCRouterConfig      `mapstructure:"mcrouter"`
	Infrared      InfraredConfig      `mapstructure:"infrared"`
	Retry         RetryConfig         `mapstructure:"retry"`
	Secrets       SecretsConfig       `mapstructure:"secrets"`
	DashboardAuth DashboardAuthConfig `mapstructure:"dashboard_auth"`
	Logging       LoggingConfig       `mapstructure:"logging"`
}
