package config

type Config struct {
	Server ServerConfig `mapstructure:"server"`
}

type ServerConfig struct {
	Address string `mapstructure:"address"`
}