package setup

import (
	"context"
	"fmt"
	"time"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
	"github.com/zurco34/pelican-mc-router/internal/retry"
)

const defaultPelicanValidationTimeout = 15 * time.Second

type PelicanClientFactory interface {
	New(config pelican.Config) (PelicanNodeLister, error)
}

type PelicanNodeLister interface {
	ListNodes(context.Context) ([]pelican.NodeResource, error)
}

type PelicanValidatorAdapter struct {
	factory PelicanClientFactory
	timeout time.Duration
	retry   retry.Config
}

func NewPelicanValidator(
	factory PelicanClientFactory,
	timeout time.Duration,
	retryConfigs ...retry.Config,
) *PelicanValidatorAdapter {
	if timeout <= 0 {
		timeout = defaultPelicanValidationTimeout
	}

	var retryConfig retry.Config
	if len(retryConfigs) > 0 {
		retryConfig = retryConfigs[0]
	}

	return &PelicanValidatorAdapter{
		factory: factory,
		timeout: timeout,
		retry:   retryConfig,
	}
}

func (v *PelicanValidatorAdapter) Validate(
	ctx context.Context,
	baseURL string,
	apiKey string,
) error {
	client, err := v.factory.New(pelican.Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Timeout: v.timeout,
		Retry:   v.retry,
	})
	if err != nil {
		return fmt.Errorf("setup: create Pelican client: %w", err)
	}

	if _, err := client.ListNodes(ctx); err != nil {
		return fmt.Errorf("setup: validate Pelican connection: %w", err)
	}

	return nil
}
