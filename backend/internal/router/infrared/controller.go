package infrared

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zurco34/pelican-mc-router/internal/router"
)

const managedProxyPrefix = "pelican-mc-router-"

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
	errDuplicateServerID = errors.New(
		"infrared: duplicate route server ID",
	)
)

type Config struct {
	Directory string
}

type Controller struct {
	mu        sync.Mutex
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
	_, err := c.ReconcileChanges(ctx, routes)

	return err
}

func (c *Controller) ReconcileChanges(
	ctx context.Context,
	routes []router.Route,
) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf(
			"infrared: reconcile routes: %w",
			err,
		)
	}

	desiredFiles := make(map[string]struct{}, len(routes))
	configurations := make(
		[]renderedProxyConfiguration,
		0,
		len(routes),
	)

	for _, route := range routes {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf(
				"infrared: reconcile routes: %w",
				err,
			)
		}

		filename, err := proxyFilename(route.ServerID)
		if err != nil {
			return false, fmt.Errorf(
				"infrared: build filename for server %q: %w",
				route.ServerID,
				err,
			)
		}

		if _, exists := desiredFiles[filename]; exists {
			return false, fmt.Errorf(
				"infrared: duplicate route for server %q: %w",
				strings.TrimSpace(route.ServerID),
				errDuplicateServerID,
			)
		}

		desiredFiles[filename] = struct{}{}

		data, err := Render(route)
		if err != nil {
			return false, fmt.Errorf(
				"infrared: render route for server %q: %w",
				route.ServerID,
				err,
			)
		}

		configurations = append(
			configurations,
			renderedProxyConfiguration{
				filename: filename,
				data:     data,
			},
		)
	}

	if err := os.MkdirAll(c.directory, 0o755); err != nil {
		return false, fmt.Errorf(
			"infrared: create proxy directory %q: %w",
			c.directory,
			err,
		)
	}

	changed := false

	for _, configuration := range configurations {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf(
				"infrared: reconcile routes: %w",
				err,
			)
		}

		path := filepath.Join(
			c.directory,
			configuration.filename,
		)

		needsUpdate, err := proxyConfigurationNeedsUpdate(
			path,
			configuration.data,
			0o644,
		)
		if err != nil {
			return false, fmt.Errorf(
				"infrared: inspect proxy configuration %q: %w",
				path,
				err,
			)
		}

		if !needsUpdate {
			continue
		}

		if err := writeFileAtomic(
			path,
			configuration.data,
			0o644,
		); err != nil {
			return false, fmt.Errorf(
				"infrared: write proxy configuration %q: %w",
				path,
				err,
			)
		}

		changed = true
	}

	staleRemoved, err := removeStaleProxyConfigurations(
		ctx,
		c.directory,
		desiredFiles,
	)
	if err != nil {
		return false, err
	}

	return changed || staleRemoved, nil
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

	return managedProxyPrefix + serverID + ".yml", nil
}

func proxyConfigurationNeedsUpdate(
	path string,
	data []byte,
	permissions os.FileMode,
) (bool, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return true, nil
	case err != nil:
		return false, fmt.Errorf(
			"inspect existing file: %w",
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return true, nil
	}

	existing, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf(
			"read existing file: %w",
			err,
		)
	}

	if !bytes.Equal(existing, data) {
		return true, nil
	}

	return info.Mode().Perm() != permissions.Perm(), nil
}

func removeStaleProxyConfigurations(
	ctx context.Context,
	directory string,
	desiredFiles map[string]struct{},
) (bool, error) {
	entries, err := os.ReadDir(directory)

	if err != nil {
		return false, fmt.Errorf(
			"infrared: read proxy directory %q: %w",
			directory,
			err,
		)
	}

	changed := false

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return false, fmt.Errorf("infrared: reconcile routes: %w", err)
		}

		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasPrefix(name, managedProxyPrefix) ||
			filepath.Ext(name) != ".yml" {
			continue
		}

		if _, exists := desiredFiles[name]; exists {
			continue
		}

		path := filepath.Join(directory, name)

		if err := os.Remove(path); err != nil {
			return false, fmt.Errorf(
				"infrared: remove stale proxy configuration %q: %w",
				path,
				err,
			)
		}

		changed = true

	}

	return changed, nil
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
