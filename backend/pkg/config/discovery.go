package config

import "time"

type DiscoveryConfig struct {
	Interval time.Duration `mapstructure:"interval"`
}
