package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	routerBackendMCRouter = "mc-router"
	routerBackendInfrared = "infrared"
)

var (
	ErrInvalidServerPort               = errors.New("server port must be between 1 and 65535")
	ErrInvalidServerReadHeaderTimeout  = errors.New("server read header timeout must be greater than zero")
	ErrInvalidServerReadTimeout        = errors.New("server read timeout must be greater than zero")
	ErrInvalidServerWriteTimeout       = errors.New("server write timeout must be greater than zero")
	ErrInvalidServerIdleTimeout        = errors.New("server idle timeout must be greater than zero")
	ErrInvalidRetryAttempts            = errors.New("retry attempts must be greater than zero")
	ErrInvalidRetryInitialBackoff      = errors.New("retry initial backoff must be greater than zero")
	ErrInvalidRetryMaxBackoff          = errors.New("retry max backoff must be greater than or equal to initial backoff")
	ErrMissingPelicanURL               = errors.New("pelican URL is required")
	ErrInvalidPelicanURL               = errors.New("pelican URL must be a valid HTTP or HTTPS URL")
	ErrMissingPelicanAPIKey            = errors.New("pelican API key is required")
	ErrInvalidPelicanTimeout           = errors.New("pelican timeout must be greater than zero")
	ErrInvalidDiscoveryInterval        = errors.New("discovery interval must be greater than zero")
	ErrMissingRouterDomain             = errors.New("router domain is required")
	ErrMissingInfraredProxiesPath      = errors.New("infrared proxies path is required")
	ErrMissingInfraredReloadMarkerPath = errors.New("infrared reload marker path is required")
	ErrUnsupportedRouterBackend        = errors.New("router backend is unsupported")
	ErrMissingMCRouterAPIURL           = errors.New("mc-router API URL is required")
	ErrInvalidMCRouterAPIURL           = errors.New("mc-router API URL must be a valid HTTP or HTTPS URL")
	ErrMissingDatabasePath             = errors.New("database path is required")
)

func (c Config) ValidateInfrastructure() error {
	var validationErrors []error

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		validationErrors = append(
			validationErrors,
			ErrInvalidServerPort,
		)
	}
	if c.Server.ReadHeaderTimeout <= 0 {
		validationErrors = append(validationErrors, ErrInvalidServerReadHeaderTimeout)
	}
	if c.Server.ReadTimeout <= 0 {
		validationErrors = append(validationErrors, ErrInvalidServerReadTimeout)
	}
	if c.Server.WriteTimeout <= 0 {
		validationErrors = append(validationErrors, ErrInvalidServerWriteTimeout)
	}
	if c.Server.IdleTimeout <= 0 {
		validationErrors = append(validationErrors, ErrInvalidServerIdleTimeout)
	}

	if strings.TrimSpace(c.Database.Path) == "" {
		validationErrors = append(
			validationErrors,
			ErrMissingDatabasePath,
		)
	}

	if err := validateRouterInfrastructure(c); err != nil {
		validationErrors = append(validationErrors, err)
	}

	if c.Discovery.Interval <= 0 {
		validationErrors = append(
			validationErrors,
			ErrInvalidDiscoveryInterval,
		)
	}
	if c.Retry.Attempts <= 0 {
		validationErrors = append(validationErrors, ErrInvalidRetryAttempts)
	}
	if c.Retry.InitialBackoff <= 0 {
		validationErrors = append(validationErrors, ErrInvalidRetryInitialBackoff)
	}
	if c.Retry.MaxBackoff < c.Retry.InitialBackoff {
		validationErrors = append(validationErrors, ErrInvalidRetryMaxBackoff)
	}

	return errors.Join(validationErrors...)
}

func validateRouterInfrastructure(cfg Config) error {
	switch strings.TrimSpace(cfg.Router.Backend) {
	case routerBackendMCRouter:
		return validateMCRouterAPIURL(cfg.MCRouter.APIURL)

	case routerBackendInfrared:
		var validationErrors []error

		if strings.TrimSpace(cfg.Infrared.ProxiesPath) == "" {
			validationErrors = append(
				validationErrors,
				ErrMissingInfraredProxiesPath,
			)
		}

		if strings.TrimSpace(cfg.Infrared.ReloadMarkerPath) == "" {
			validationErrors = append(
				validationErrors,
				ErrMissingInfraredReloadMarkerPath,
			)
		}

		return errors.Join(validationErrors...)

	default:
		return fmt.Errorf(
			"%w: %q",
			ErrUnsupportedRouterBackend,
			cfg.Router.Backend,
		)
	}
}

func validateMCRouterAPIURL(value string) error {
	rawURL := strings.TrimSpace(value)
	if rawURL == "" {
		return ErrMissingMCRouterAPIURL
	}

	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil ||
		(parsedURL.Scheme != "http" &&
			parsedURL.Scheme != "https") ||
		parsedURL.Host == "" {
		return fmt.Errorf(
			"%w: %q",
			ErrInvalidMCRouterAPIURL,
			rawURL,
		)
	}

	return nil
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
