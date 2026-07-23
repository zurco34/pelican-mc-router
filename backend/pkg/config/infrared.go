package config

type InfraredConfig struct {
	ProxiesPath  string `mapstructure:"proxies_path"`
	ReloadSignal string `mapstructure:"reload_signal"`
}
