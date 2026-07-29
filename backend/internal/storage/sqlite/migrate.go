package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/zurco34/pelican-mc-router/internal/storage/migrations"
)

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			checksum TEXT NOT NULL DEFAULT '',
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT ''`); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("add migration checksum column: %w", err)
	}

	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		contents, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
		applied, recorded, err := migrationApplied(db, entry.Name())
		if err != nil {
			return err
		}
		if applied {
			if recorded != "" && recorded != checksum {
				return fmt.Errorf("migration checksum mismatch for %q", entry.Name())
			}
			if recorded == "" {
				if _, err := db.Exec(`UPDATE schema_migrations SET checksum = ? WHERE version = ?`, checksum, entry.Name()); err != nil {
					return fmt.Errorf("record migration checksum %q: %w", entry.Name(), err)
				}
			}
			continue
		}
		if err := applyMigration(db, entry.Name(), string(contents), checksum); err != nil {
			return err
		}
	}

	return nil
}

func migrationApplied(db *sql.DB, version string) (bool, string, error) {
	var exists int

	err := db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM schema_migrations WHERE version = ?
		)`,
		version,
	).Scan(&exists)
	if err != nil {
		return false, "", fmt.Errorf("check migration %q: %w", version, err)
	}

	if exists == 0 {
		return false, "", nil
	}
	var checksum string
	if err := db.QueryRow(`SELECT checksum FROM schema_migrations WHERE version = ?`, version).Scan(&checksum); err != nil {
		return false, "", fmt.Errorf("read migration checksum %q: %w", version, err)
	}
	return true, checksum, nil
}

func applyMigration(db *sql.DB, version, contents, checksum string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", version, err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(contents); err != nil {
		return fmt.Errorf("apply migration %q: %w", version, err)
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, checksum) VALUES (?, ?)`, version, checksum,
	); err != nil {
		return fmt.Errorf("record migration %q: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", version, err)
	}

	return nil
}
