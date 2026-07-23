package infrared

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

func TestControllerReconcileWritesProxyConfiguration(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	controller, err := NewController(Config{
		Directory: directory,
	})
	if err != nil {
		t.Fatalf("NewController() error = %v", err)
	}

	route := router.Route{
		ServerID: "server-123",
		Hostname: "atm10.mc.example.com",
		Backend: router.Backend{
			Host: "10.0.0.25",
			Port: 25565,
		},
	}

	err = controller.Reconcile(
		context.Background(),
		[]router.Route{route},
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	path := filepath.Join(directory, "server-123.yml")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read proxy configuration: %v", err)
	}

	want, err := Render(route)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf(
			"proxy configuration =\n%s\nwant:\n%s",
			string(got),
			string(want),
		)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read proxy directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf(
			"proxy directory contains %d entries, want 1",
			len(entries),
		)
	}

	if entries[0].Name() != "server-123.yml" {
		t.Fatalf(
			"proxy filename = %q, want %q",
			entries[0].Name(),
			"server-123.yml",
		)
	}
}

func TestNewControllerRejectsEmptyDirectory(t *testing.T) {
	t.Parallel()

	_, err := NewController(Config{
		Directory: "   ",
	})
	if !errors.Is(err, errEmptyDirectory) {
		t.Fatalf(
			"NewController() error = %v, want %v",
			err,
			errEmptyDirectory,
		)
	}
}

func TestControllerReconcileRejectsInvalidServerID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		serverID string
		wantErr  error
	}{
		{
			name:     "empty",
			serverID: "",
			wantErr:  errEmptyServerID,
		},
		{
			name:     "current directory",
			serverID: ".",
			wantErr:  errInvalidServerID,
		},
		{
			name:     "parent directory",
			serverID: "..",
			wantErr:  errInvalidServerID,
		},
		{
			name:     "forward slash",
			serverID: "../outside",
			wantErr:  errInvalidServerID,
		},
		{
			name:     "backslash",
			serverID: `..\outside`,
			wantErr:  errInvalidServerID,
		},
		{
			name:     "null byte",
			serverID: "server\x00outside",
			wantErr:  errInvalidServerID,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			controller, err := NewController(Config{
				Directory: t.TempDir(),
			})
			if err != nil {
				t.Fatalf("NewController() error = %v", err)
			}

			route := router.Route{
				ServerID: test.serverID,
				Hostname: "server.mc.example.com",
				Backend: router.Backend{
					Host: "10.0.0.25",
					Port: 25565,
				},
			}

			err = controller.Reconcile(
				context.Background(),
				[]router.Route{route},
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"Reconcile() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestControllerReconcileReplacesExistingConfiguration(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()

	controller, err := NewController(Config{
		Directory: directory,
	})
	if err != nil {
		t.Fatalf("NewController() error = %v", err)
	}

	path := filepath.Join(directory, "server-123.yml")

	if err := os.WriteFile(path, []byte("old configuration\n"), 0o644); err != nil {
		t.Fatalf("write existing proxy configuration: %v", err)
	}

	route := router.Route{
		ServerID: "server-123",
		Hostname: "updated.mc.example.com",
		Backend: router.Backend{
			Host: "10.0.0.50",
			Port: 25570,
		},
	}

	err = controller.Reconcile(
		context.Background(),
		[]router.Route{route},
	)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced proxy configuration: %v", err)
	}

	want, err := Render(route)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if string(got) != string(want) {
		t.Fatalf(
			"replaced proxy configuration =\n%s\nwant:\n%s",
			string(got),
			string(want),
		)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read proxy directory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf(
			"proxy directory contains %d entries, want 1",
			len(entries),
		)
	}

	if entries[0].Name() != "server-123.yml" {
		t.Fatalf(
			"proxy directory entry = %q, want %q",
			entries[0].Name(),
			"server-123.yml",
		)
	}
}
