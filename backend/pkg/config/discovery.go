package config

import "time"

type DiscoveryConfig struct {
	Interval            time.Duration `mapstructure:"interval"`
	WildcardBackendHost string        `mapstructure:"wildcard_backend_host"`
}
