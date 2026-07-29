package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const defaultConfigName = "config"

func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)

	v.SetConfigName(defaultConfigName)
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	v.SetEnvPrefix("PELICAN_MC_ROUTER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError

		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("config: read configuration: %w", err)
		}
	}

	var cfg Config

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config: decode configuration: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_header_timeout", 5*time.Second)
	v.SetDefault("server.read_timeout", 15*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.idle_timeout", time.Minute)

	v.SetDefault(
		"database.path",
		"./data/pelican-mc-router.db",
	)

	v.SetDefault("pelican.timeout", 15*time.Second)
	v.SetDefault("retry.attempts", 3)
	v.SetDefault("retry.initial_backoff", 200*time.Millisecond)
	v.SetDefault("retry.max_backoff", 2*time.Second)

	v.SetDefault("dashboard_auth.enabled", false)
	v.SetDefault("dashboard_auth.role_claim", "roles")
	v.SetDefault("dashboard_auth.required_role", "viewer")
	v.SetDefault("dashboard_auth.discovery_timeout", 5*time.Second)

	v.SetDefault("discovery.interval", 30*time.Second)
	v.SetDefault("discovery.wildcard_backend_host", "")

	v.SetDefault("router.backend", "mc-router")

	v.SetDefault("mcrouter.api_url", "http://mc-router:8080")

	v.SetDefault("infrared.proxies_path", "/etc/infrared/proxies")

	v.SetDefault(
		"infrared.reload_marker_path",
		"/etc/infrared/control/infrared.reload",
	)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}
