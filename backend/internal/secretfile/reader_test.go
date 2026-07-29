package secretfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReaderRead(t *testing.T) {
	directory := t.TempDir()
	writeSecret(t, directory, "pelican-token", []byte("value"), 0o600)

	reader, err := New(directory)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	value, err := reader.Read("pelican-token")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	defer clear(value)
	if got, want := string(value), "value"; got != want {
		t.Errorf("Read() = %q, want %q", got, want)
	}
}

func TestReaderReadRejectsUnsafeInputs(t *testing.T) {
	directory := t.TempDir()
	writeSecret(t, directory, "private", []byte("value"), 0o600)
	writeSecret(t, directory, "readable", []byte("value"), 0o640)
	writeSecret(t, directory, "not-readable", []byte("value"), 0o200)
	if err := os.Mkdir(filepath.Join(directory, "not-a-file"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(directory, "private"), filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}

	reader, err := New(directory)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  error
	}{
		{name: "path traversal", input: "../private", want: ErrInvalidName},
		{name: "path separator", input: "nested/private", want: ErrInvalidName},
		{name: "missing", input: "missing", want: ErrUnavailable},
		{name: "symlink", input: "link", want: ErrNotRegular},
		{name: "directory", input: "not-a-file", want: ErrNotRegular},
		{name: "group readable", input: "readable", want: ErrInsecureMode},
		{name: "owner not readable", input: "not-readable", want: ErrInsecureMode},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := reader.Read(test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("Read() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewRejectsMissingAndSymlinkDirectories(t *testing.T) {
	directory := t.TempDir()
	if _, err := New(filepath.Join(directory, "missing")); !errors.Is(err, ErrInvalidDirectory) {
		t.Fatalf("New(missing) error = %v, want %v", err, ErrInvalidDirectory)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(directory, link); err != nil {
		t.Fatal(err)
	}
	if _, err := New(link); !errors.Is(err, ErrInvalidDirectory) {
		t.Fatalf("New(symlink) error = %v, want %v", err, ErrInvalidDirectory)
	}
}

func TestReaderReadRejectsOversizedFile(t *testing.T) {
	directory := t.TempDir()
	writeSecret(t, directory, "large", make([]byte, maxSecretSize+1), 0o600)
	reader, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read("large"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Read() error = %v, want %v", err, ErrTooLarge)
	}
}

func writeSecret(t *testing.T, directory, name string, value []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), value, mode); err != nil {
		t.Fatal(err)
	}
}
