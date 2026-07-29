// Package secretfile reads deployment-mounted secrets through constrained names.
package secretfile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

const maxSecretSize = 64 * 1024

var (
	ErrInvalidDirectory = errors.New("secret directory is invalid")
	ErrInvalidName      = errors.New("secret name is invalid")
	ErrUnavailable      = errors.New("secret is unavailable")
	ErrNotRegular       = errors.New("secret must be a regular file")
	ErrInsecureMode     = errors.New("secret file permissions are insecure")
	ErrTooLarge         = errors.New("secret file exceeds the maximum size")
	validName           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

// Reader restricts secret reads to regular, owner-only files in one directory.
// It returns bytes so callers can clear them after use.
type Reader struct {
	directory string
}

func New(directory string) (*Reader, error) {
	info, err := os.Lstat(directory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, ErrInvalidDirectory
	}

	return &Reader{directory: directory}, nil
}

// Read returns a mounted secret. Names never contain paths, and symlinks,
// non-regular files, group-readable files, and world-readable files are refused.
func (r *Reader) Read(name string) ([]byte, error) {
	if r == nil || !validName.MatchString(name) {
		return nil, ErrInvalidName
	}

	path := filepath.Join(r.directory, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	if !isSafeFile(info) {
		return nil, ErrNotRegular
	}
	if !hasSafePermissions(info) {
		return nil, ErrInsecureMode
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer file.Close()

	openedInfo, err := file.Stat()
	if err != nil || !isSafeFile(openedInfo) || !os.SameFile(info, openedInfo) {
		return nil, ErrNotRegular
	}
	if !hasSafePermissions(openedInfo) {
		return nil, ErrInsecureMode
	}

	value, err := io.ReadAll(io.LimitReader(file, maxSecretSize+1))
	if err != nil {
		return nil, ErrUnavailable
	}
	if len(value) > maxSecretSize {
		clear(value)
		return nil, ErrTooLarge
	}

	return value, nil
}

func isSafeFile(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular()
}

func hasSafePermissions(info os.FileInfo) bool {
	permissions := info.Mode().Perm()
	return permissions&0o400 != 0 && permissions&0o077 == 0
}
