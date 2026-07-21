package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Config struct {
	Path string
}

func Open(cfg Config) (*sql.DB, error) {
	if cfg.Path == "" {
		return nil, errors.New("database path is required")
	}

	parentDir := filepath.Dir(cfg.Path)

	if err := os.MkdirAll(parentDir, 0o750); err != nil {
		return nil, fmt.Errorf("create database directory %q: %w", parentDir, err)
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping SQLite database: %w", err)
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate SQLite database: %w", err)
	}

	return db, nil
}
