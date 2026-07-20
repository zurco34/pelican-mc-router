package config

import "time"

type PelicanConfig struct {
	URL     string        `mapstructure:"url"`
	APIKey  string        `mapstructure:"api_key"`
	Timeout time.Duration `mapstructure:"timeout"`
}
