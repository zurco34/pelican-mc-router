package setup

import (
	"context"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
)

type factoryStubPelicanNodeLister struct{}

func (factoryStubPelicanNodeLister) ListNodes(
	context.Context,
) ([]pelican.NodeResource, error) {
	return nil, nil
}

func TestPelicanClientFactoryFuncNew(t *testing.T) {
	wantClient := factoryStubPelicanNodeLister{}

	var received pelican.Config

	factory := PelicanClientFactoryFunc(
		func(cfg pelican.Config) (PelicanNodeLister, error) {
			received = cfg
			return wantClient, nil
		},
	)

	wantConfig := pelican.Config{
		BaseURL: "https://panel.example.com",
		APIKey:  "test-key",
	}

	client, err := factory.New(wantConfig)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client == nil {
		t.Fatal("New() client = nil, want a client")
	}

	if received.BaseURL != wantConfig.BaseURL {
		t.Errorf(
			"config BaseURL = %q, want %q",
			received.BaseURL,
			wantConfig.BaseURL,
		)
	}

	if received.APIKey != wantConfig.APIKey {
		t.Errorf(
			"config APIKey = %q, want %q",
			received.APIKey,
			wantConfig.APIKey,
		)
	}
}
