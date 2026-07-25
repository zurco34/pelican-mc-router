package infrared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errEmptyReloadMarkerPath = errors.New(
	"infrared: reload marker path must not be empty",
)

type MarkerReloader struct {
	path string
	now  func() time.Time
}

func NewMarkerReloader(path string) (*MarkerReloader, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errEmptyReloadMarkerPath
	}

	return &MarkerReloader{
		path: filepath.Clean(path),
		now:  time.Now,
	}, nil
}

func (r *MarkerReloader) Reload(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"infrared: update reload marker: %w",
			err,
		)
	}

	directory := filepath.Dir(r.path)

	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf(
			"infrared: create reload marker directory %q: %w",
			directory,
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"infrared: update reload marker: %w",
			err,
		)
	}

	data := []byte(
		r.now().UTC().Format(time.RFC3339Nano) + "\n",
	)

	if err := writeFileAtomic(
		r.path,
		data,
		0o644,
	); err != nil {
		return fmt.Errorf(
			"infrared: write reload marker %q: %w",
			r.path,
			err,
		)
	}

	return nil
}
