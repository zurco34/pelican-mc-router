package sqlite

import (
	"path/filepath"
	"strings"
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

func TestMigrateRecordsMigrationChecksums(t *testing.T) {
	db, err := Open(Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var checksum string
	if err := db.QueryRow(`SELECT checksum FROM schema_migrations WHERE version = ?`, "0001_initial.sql").Scan(&checksum); err != nil {
		t.Fatalf("query checksum: %v", err)
	}
	if checksum == "" {
		t.Fatal("migration checksum is empty")
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET checksum = 'changed' WHERE version = ?`, "0001_initial.sql"); err != nil {
		t.Fatalf("change checksum: %v", err)
	}
	if err := Migrate(db); err == nil {
		t.Fatal("Migrate() error = nil, want checksum mismatch")
	}
}

func TestMigrateRejectsFutureSchemaBeforeMetadataWrites(t *testing.T) {
	db, err := Open(Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, checksum) VALUES ('9999_future.sql', 'future-checksum')`); err != nil {
		t.Fatal(err)
	}
	var before int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	err = Migrate(db)
	if err == nil || !strings.Contains(err.Error(), "database schema is newer than this binary") {
		t.Fatalf("Migrate() error = %v, want future schema refusal", err)
	}
	var after int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("migration metadata count = %d, want unchanged %d", after, before)
	}
}
