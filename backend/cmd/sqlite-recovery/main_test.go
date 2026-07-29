package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDatabaseCreatesPrivateDistinctBackup(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.db")
	if err := os.WriteFile(source, []byte("sqlite-data"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	target := filepath.Join(t.TempDir(), "backup", "router.db")
	copyDatabase(source, target)
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(contents) != "sqlite-data" {
		t.Fatalf("backup contents = %q", contents)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target permissions = %o, want 600", info.Mode().Perm())
	}
}
