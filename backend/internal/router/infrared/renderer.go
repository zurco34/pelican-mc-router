package infrared

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"go.yaml.in/yaml/v3"
)

var (
	errEmptyHostname = errors.New(
		"infrared: route hostname must not be empty",
	)
	errEmptyBackendHost = errors.New(
		"infrared: backend host must not be empty",
	)
	errInvalidBackendPort = errors.New(
		"infrared: backend port must be between 1 and 65535",
	)
)

type proxyConfig struct {
	Domains   []string `yaml:"domains"`
	Addresses []string `yaml:"addresses"`
}

func Render(route router.Route) ([]byte, error) {
	hostname := strings.TrimSpace(route.Hostname)
	if hostname == "" {
		return nil, errEmptyHostname
	}

	backendHost := strings.TrimSpace(route.Backend.Host)
	if backendHost == "" {
		return nil, errEmptyBackendHost
	}

	if route.Backend.Port < 1 || route.Backend.Port > 65535 {
		return nil, errInvalidBackendPort
	}

	config := proxyConfig{
		Domains: []string{
			hostname,
		},
		Addresses: []string{
			net.JoinHostPort(
				backendHost,
				strconv.Itoa(route.Backend.Port),
			),
		},
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf(
			"infrared: encode proxy configuration: %w",
			err,
		)
	}

	return data, nil
}
