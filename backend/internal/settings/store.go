package settings

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

var ErrNotFound = errors.New("setting not found")

const KeySetupCompleted = "setup.completed"

type Store struct {
	db *sql.DB
}

type settingWriter interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func NewStore(db *sql.DB) *Store {
	return &Store{
		db: db,
	}
}
func (s *Store) Set(key, value string) error {
	return setValue(s.db, key, value)
}

func setValue(
	writer settingWriter,
	key string,
	value string,
) error {
	_, err := writer.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`, key, value)

	return err
}
func (s *Store) Get(key string) (string, error) {
	var value string

	err := s.db.QueryRow(
		`SELECT value FROM settings WHERE key = ?`,
		key,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}

	return value, nil
}
func (s *Store) IsSetupComplete() (bool, error) {
	value, err := s.Get(KeySetupCompleted)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	complete, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf(
			"parse setting %q as boolean: %w",
			KeySetupCompleted,
			err,
		)
	}
	return complete, nil
}

func (s *Store) SetSetupComplete(complete bool) error {
	return s.Set(KeySetupCompleted, strconv.FormatBool(complete))
}
func (s *Store) Save(settings Settings) error {
	if err := s.Set(KeyPelicanURL, settings.PelicanURL); err != nil {
		return fmt.Errorf("save %q: %w", KeyPelicanURL, err)
	}

	if err := s.Set(KeyPelicanAPIKey, settings.PelicanAPIKey); err != nil {
		return fmt.Errorf("save %q: %w", KeyPelicanAPIKey, err)
	}

	if err := s.Set(KeyRouterDomain, settings.RouterDomain); err != nil {
		return fmt.Errorf("save %q: %w", KeyRouterDomain, err)
	}

	return nil
}
func (s *Store) Load() (Settings, error) {
	var result Settings

	pelicanURL, err := s.Get(KeyPelicanURL)
	if err != nil {
		return Settings{}, fmt.Errorf("load %q: %w", KeyPelicanURL, err)
	}
	result.PelicanURL = pelicanURL

	pelicanAPIKey, err := s.Get(KeyPelicanAPIKey)
	if err != nil {
		return Settings{}, fmt.Errorf("load %q: %w", KeyPelicanAPIKey, err)
	}
	result.PelicanAPIKey = pelicanAPIKey

	routerDomain, err := s.Get(KeyRouterDomain)
	if err != nil {
		return Settings{}, fmt.Errorf("load %q: %w", KeyRouterDomain, err)
	}
	result.RouterDomain = routerDomain

	return result, nil
}
func (s *Store) SaveSetup(value Settings) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin setup settings transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	values := []struct {
		key   string
		value string
	}{
		{
			key:   KeyPelicanURL,
			value: value.PelicanURL,
		},
		{
			key:   KeyPelicanAPIKey,
			value: value.PelicanAPIKey,
		},
		{
			key:   KeyRouterDomain,
			value: value.RouterDomain,
		},
		{
			key:   KeySetupCompleted,
			value: strconv.FormatBool(true),
		},
	}

	for _, setting := range values {
		if err := setValue(tx, setting.key, setting.value); err != nil {
			return fmt.Errorf("save %q: %w", setting.key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit setup settings transaction: %w", err)
	}

	return nil
}
