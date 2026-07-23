package infrared

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

var (
	errEmptyDirectory = errors.New(
		"infrared: proxy directory must not be empty",
	)
	errEmptyServerID = errors.New(
		"infrared: route server ID must not be empty",
	)
	errInvalidServerID = errors.New(
		"infrared: route server ID must be a filename-safe value",
	)
)

type Config struct {
	Directory string
}

type Controller struct {
	directory string
}

type renderedProxyConfiguration struct {
	filename string
	data     []byte
}

func NewController(config Config) (*Controller, error) {
	directory := strings.TrimSpace(config.Directory)
	if directory == "" {
		return nil, errEmptyDirectory
	}

	return &Controller{
		directory: filepath.Clean(directory),
	}, nil
}

func (c *Controller) Reconcile(
	ctx context.Context,
	routes []router.Route,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("infrared: reconcile routes: %w", err)
	}

	desiredFiles := make(map[string]struct{}, len(routes))
	configurations := make(
		[]renderedProxyConfiguration,
		0,
		len(routes),
	)

	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("infrared: reconcile routes: %w", err)
		}

		filename, err := proxyFilename(route.ServerID)
		if err != nil {
			return fmt.Errorf(
				"infrared: build filename for server %q: %w",
				route.ServerID,
				err,
			)
		}

		data, err := Render(route)
		if err != nil {
			return fmt.Errorf(
				"infrared: render route for server %q: %w",
				route.ServerID,
				err,
			)
		}

		desiredFiles[filename] = struct{}{}

		configurations = append(
			configurations,
			renderedProxyConfiguration{
				filename: filename,
				data:     data,
			},
		)
	}

	if err := os.MkdirAll(c.directory, 0o755); err != nil {
		return fmt.Errorf(
			"infrared: create proxy directory %q: %w",
			c.directory,
			err,
		)
	}

	for _, configuration := range configurations {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("infrared: reconcile routes: %w", err)
		}

		path := filepath.Join(
			c.directory,
			configuration.filename,
		)

		if err := writeFileAtomic(
			path,
			configuration.data,
			0o644,
		); err != nil {
			return fmt.Errorf(
				"infrared: write proxy configuration %q: %w",
				path,
				err,
			)
		}
	}

	if err := removeStaleProxyConfigurations(
		ctx,
		c.directory,
		desiredFiles,
	); err != nil {
		return err
	}

	return nil
}

func proxyFilename(serverID string) (string, error) {
	serverID = strings.TrimSpace(serverID)

	if serverID == "" {
		return "", errEmptyServerID
	}

	if serverID == "." ||
		serverID == ".." ||
		strings.ContainsAny(serverID, `/\`) ||
		strings.ContainsRune(serverID, '\x00') {
		return "", errInvalidServerID
	}

	return serverID + ".yml", nil
}

func removeStaleProxyConfigurations(
	ctx context.Context,
	directory string,
	desiredFiles map[string]struct{},
) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf(
			"infrared: read proxy directory %q: %w",
			directory,
			err,
		)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("infrared: reconcile routes: %w", err)
		}

		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if filepath.Ext(name) != ".yml" {
			continue
		}

		if _, exists := desiredFiles[name]; exists {
			continue
		}

		path := filepath.Join(directory, name)

		if err := os.Remove(path); err != nil {
			return fmt.Errorf(
				"infrared: remove stale proxy configuration %q: %w",
				path,
				err,
			)
		}
	}

	return nil
}

func writeFileAtomic(
	path string,
	data []byte,
	permissions os.FileMode,
) error {
	directory := filepath.Dir(path)

	file, err := os.CreateTemp(
		directory,
		"."+filepath.Base(path)+".tmp-*",
	)
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	temporaryPath := file.Name()

	defer func() {
		_ = file.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := file.Chmod(permissions); err != nil {
		return fmt.Errorf("set temporary file permissions: %w", err)
	}

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temporary file: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}

	return nil
}
