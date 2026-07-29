package config

import "time"

type RetryConfig struct {
	Attempts       int           `mapstructure:"attempts"`
	InitialBackoff time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff     time.Duration `mapstructure:"max_backoff"`
}
