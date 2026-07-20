package config

type RouterConfig struct {
	Backend string `mapstructure:"backend"`
	Domain  string `mapstructure:"domain"`
}
