package config

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Pelican   PelicanConfig   `mapstructure:"pelican"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Router    RouterConfig    `mapstructure:"router"`
	Infrared  InfraredConfig  `mapstructure:"infrared"`
	Logging   LoggingConfig   `mapstructure:"logging"`
}
