package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var (
	ErrInvalidServerPort          = errors.New("server port must be between 1 and 65535")
	ErrMissingPelicanURL          = errors.New("pelican URL is required")
	ErrInvalidPelicanURL          = errors.New("pelican URL must be a valid HTTP or HTTPS URL")
	ErrMissingPelicanAPIKey       = errors.New("pelican API key is required")
	ErrInvalidPelicanTimeout      = errors.New("pelican timeout must be greater than zero")
	ErrInvalidDiscoveryInterval   = errors.New("discovery interval must be greater than zero")
	ErrMissingRouterDomain        = errors.New("router domain is required")
	ErrMissingInfraredProxiesPath = errors.New("infrared proxies path is required")
)

var ErrMissingDatabasePath = errors.New("database path is required")

func (c Config) ValidateInfrastructure() error {
	var validationErrors []error

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		validationErrors = append(
			validationErrors,
			ErrInvalidServerPort,
		)
	}

	if strings.TrimSpace(c.Database.Path) == "" {
		validationErrors = append(
			validationErrors,
			ErrMissingDatabasePath,
		)
	}

	if strings.TrimSpace(c.Infrared.ProxiesPath) == "" {
		validationErrors = append(
			validationErrors,
			ErrMissingInfraredProxiesPath,
		)
	}

	if c.Discovery.Interval <= 0 {
		validationErrors = append(
			validationErrors,
			ErrInvalidDiscoveryInterval,
		)
	}

	return errors.Join(validationErrors...)
}

func (c Config) Validate() error {
	var validationErrors []error

	if err := c.ValidateInfrastructure(); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if err := validatePelican(c.Pelican); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if strings.TrimSpace(c.Router.Domain) == "" {
		validationErrors = append(
			validationErrors,
			ErrMissingRouterDomain,
		)
	}

	return errors.Join(validationErrors...)
}

func validatePelican(cfg PelicanConfig) error {
	var validationErrors []error

	rawURL := strings.TrimSpace(cfg.URL)

	if rawURL == "" {
		validationErrors = append(
			validationErrors,
			ErrMissingPelicanURL,
		)
	} else {
		parsedURL, err := url.ParseRequestURI(rawURL)
		if err != nil ||
			(parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
			parsedURL.Host == "" {
			validationErrors = append(
				validationErrors,
				fmt.Errorf(
					"%w: %q",
					ErrInvalidPelicanURL,
					rawURL,
				),
			)
		}
	}

	if strings.TrimSpace(cfg.APIKey) == "" {
		validationErrors = append(
			validationErrors,
			ErrMissingPelicanAPIKey,
		)
	}

	if cfg.Timeout <= 0 {
		validationErrors = append(
			validationErrors,
			ErrInvalidPelicanTimeout,
		)
	}

	return errors.Join(validationErrors...)
}
