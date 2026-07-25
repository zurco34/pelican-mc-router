package config

type InfraredConfig struct {
	ProxiesPath      string `mapstructure:"proxies_path"`
	ReloadMarkerPath string `mapstructure:"reload_marker_path"`
}
