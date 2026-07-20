package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/pelican"
)

type fakePelicanClient struct {
	servers    []pelican.ServerResource
	serversErr error
	eggs       []pelican.EggResource
	eggsErr    error
}

func (f *fakePelicanClient) ListServers(
	context.Context,
) ([]pelican.ServerResource, error) {
	return f.servers, f.serversErr
}

func (f *fakePelicanClient) ListEggs(
	context.Context,
) ([]pelican.EggResource, error) {
	return f.eggs, f.eggsErr
}
func TestServiceDiscoverFiltersMinecraftServers(t *testing.T) {
	client := &fakePelicanClient{
		servers: []pelican.ServerResource{
			{
				Attributes: pelican.ServerAttributes{
					ID:   1,
					Name: "Factorio",
					Egg:  1,
				},
			},
			{
				Attributes: pelican.ServerAttributes{
					ID:   2,
					Name: "Vanilla",
					Egg:  3,
				},
			},
			{
				Attributes: pelican.ServerAttributes{
					ID:   3,
					Name: "CurseForge",
					Egg:  2,
				},
			},
		},
		eggs: []pelican.EggResource{
			{
				Attributes: pelican.EggAttributes{
					ID:   1,
					Tags: []string{"factorio"},
				},
			},
			{
				Attributes: pelican.EggAttributes{
					ID:   2,
					Tags: []string{"minecraft"},
				},
			},
			{
				Attributes: pelican.EggAttributes{
					ID:   3,
					Tags: []string{"minecraft"},
				},
			},
		},
	}

	service := New(client)

	got, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("Discover() returned %d servers, want 2", len(got))
	}

	if got[0].ID != 2 {
		t.Errorf("first server ID = %d, want 2", got[0].ID)
	}

	if got[1].ID != 3 {
		t.Errorf("second server ID = %d, want 3", got[1].ID)
	}
}

func TestServiceDiscoverReturnsServerListError(t *testing.T) {
	expectedErr := errors.New("list servers failed")

	client := &fakePelicanClient{
		serversErr: expectedErr,
	}

	service := New(client)

	_, err := service.Discover(context.Background())
	if err == nil {
		t.Fatal("Discover() error = nil, want error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Discover() error = %v, want wrapped %v", err, expectedErr)
	}
}

func TestServiceDiscoverReturnsEggListError(t *testing.T) {
	expectedErr := errors.New("list eggs failed")

	client := &fakePelicanClient{
		eggsErr: expectedErr,
	}

	service := New(client)

	_, err := service.Discover(context.Background())
	if err == nil {
		t.Fatal("Discover() error = nil, want error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Discover() error = %v, want wrapped %v", err, expectedErr)
	}
}
func TestServiceDiscoverSkipsServerWithUnknownEgg(t *testing.T) {
	client := &fakePelicanClient{
		servers: []pelican.ServerResource{
			{
				Attributes: pelican.ServerAttributes{
					ID:   1,
					Name: "Unknown",
					Egg:  999,
				},
			},
		},
		eggs: []pelican.EggResource{},
	}

	service := New(client)

	got, err := service.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(got) != 0 {
		t.Errorf("Discover() returned %d servers, want 0", len(got))
	}
}
