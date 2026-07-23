package sqlite

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabase(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "data", "router.db")

	db, err := Open(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := db.Ping(); err != nil {
		t.Fatalf("database ping failed: %v", err)
	}
}
func TestOpenRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	db, err := Open(Config{})
	if err == nil {
		if db != nil {
			_ = db.Close()
		}

		t.Fatal("Open() error = nil, want an error")
	}
}
func TestMigrateCreatesSchemaMigrationsTable(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "router.db")

	db, err := Open(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var tableName string
	err = db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'schema_migrations'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("query schema_migrations table: %v", err)
	}

	if tableName != "schema_migrations" {
		t.Fatalf("table name = %q, want %q", tableName, "schema_migrations")
	}
}
func TestMigrateRecordsAppliedMigration(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "router.db")

	db, err := Open(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var version string
	err = db.QueryRow(`
		SELECT version
		FROM schema_migrations
		WHERE version = ?
	`, "0001_initial.sql").Scan(&version)
	if err != nil {
		t.Fatalf("query applied migration: %v", err)
	}

	if version != "0001_initial.sql" {
		t.Fatalf("migration version = %q, want %q", version, "0001_initial.sql")
	}
}
func TestMigrateIsIdempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "router.db")

	db, err := Open(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var count int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE version = ?
	`, "0001_initial.sql").Scan(&count)
	if err != nil {
		t.Fatalf("count applied migration: %v", err)
	}

	if count != 1 {
		t.Fatalf("migration count = %d, want 1", count)
	}
}
func TestMigrateCreatesSettingsTable(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "router.db")

	db, err := Open(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	var tableName string
	err = db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'settings'
	`).Scan(&tableName)
	if err != nil {
		t.Fatalf("query settings table: %v", err)
	}

	if tableName != "settings" {
		t.Fatalf("table name = %q, want %q", tableName, "settings")
	}
}
