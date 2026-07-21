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

func NewStore(db *sql.DB) *Store {
	return &Store{
		db: db,
	}
}
func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(`
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
