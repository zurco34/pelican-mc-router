package infrared

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkerReloaderCreatesReloadMarker(t *testing.T) {
	t.Parallel()

	path := filepath.Join(
		t.TempDir(),
		"control",
		"infrared.reload",
	)

	reloader, err := NewMarkerReloader(path)
	if err != nil {
		t.Fatalf("NewMarkerReloader() error = %v", err)
	}

	err = reloader.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read reload marker: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("reload marker is empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat reload marker: %v", err)
	}

	if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Fatalf(
			"reload marker permissions = %o, want %o",
			got,
			want,
		)
	}
}
