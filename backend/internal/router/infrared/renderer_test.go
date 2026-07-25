package infrared

import (
	"errors"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
	"go.yaml.in/yaml/v3"
)

func TestRenderRoute(t *testing.T) {
	t.Parallel()

	route := router.Route{
		ServerID: "server-123",
		Hostname: "atm10.mc.example.com",
		Backend: router.Backend{
			Host: "10.0.0.25",
			Port: 25565,
		},
	}

	got, err := Render(route)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := "" +
		"domains:\n" +
		"    - atm10.mc.example.com\n" +
		"addresses:\n" +
		"    - 10.0.0.25:25565\n"

	if string(got) != want {
		t.Fatalf(
			"Render() =\n%s\nwant:\n%s",
			string(got),
			want,
		)
	}
}

func TestRenderRejectsInvalidRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		route   router.Route
		wantErr error
	}{
		{
			name: "empty hostname",
			route: router.Route{
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 25565,
				},
			},
			wantErr: errEmptyHostname,
		},
		{
			name: "empty backend host",
			route: router.Route{
				Hostname: "atm10.mc.example.com",
				Backend: router.Backend{
					Port: 25565,
				},
			},
			wantErr: errEmptyBackendHost,
		},
		{
			name: "zero backend port",
			route: router.Route{
				Hostname: "atm10.mc.example.com",
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 0,
				},
			},
			wantErr: errInvalidBackendPort,
		},
		{
			name: "backend port above maximum",
			route: router.Route{
				Hostname: "atm10.mc.example.com",
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 65536,
				},
			},
			wantErr: errInvalidBackendPort,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := Render(test.route)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"Render() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestRenderFormatsIPv6BackendAddress(t *testing.T) {
	t.Parallel()

	route := router.Route{
		ServerID: "server-ipv6",
		Hostname: "ipv6.mc.example.com",
		Backend: router.Backend{
			Host: "2001:db8::25",
			Port: 25565,
		},
	}

	data, err := Render(route)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var config proxyConfig

	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("decode rendered configuration: %v", err)
	}

	if len(config.Addresses) != 1 {
		t.Fatalf(
			"address count = %d, want 1",
			len(config.Addresses),
		)
	}

	const want = "[2001:db8::25]:25565"

	if config.Addresses[0] != want {
		t.Fatalf(
			"backend address = %q, want %q",
			config.Addresses[0],
			want,
		)
	}
}
