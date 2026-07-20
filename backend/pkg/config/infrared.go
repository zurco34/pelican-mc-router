package config

type InfraredConfig struct {
	ConfigPath   string `mapstructure:"config_path"`
	ReloadSignal string `mapstructure:"reload_signal"`
}
