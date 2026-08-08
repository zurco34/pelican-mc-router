package settings

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

func (s *Store) Save(value Settings) error {
	return s.save(value, false)
}

// StageSetup records a safe, retryable setup candidate. It intentionally does
// not make the candidate active or mark setup as complete.
func (s *Store) StageSetup(value Settings) (string, error) {
	if value.PelicanSecretName == "" {
		return "", errors.New("stage pending setup: Pelican secret reference is required")
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate pending setup generation: %w", err)
	}
	generation := hex.EncodeToString(bytes)
	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("begin pending setup transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
		INSERT INTO pending_setup (id, pelican_url, pelican_secret_name, router_domain, generation, updated_at)
		VALUES (1, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			pelican_url = excluded.pelican_url,
			pelican_secret_name = excluded.pelican_secret_name,
			router_domain = excluded.router_domain,
			generation = excluded.generation,
			updated_at = CURRENT_TIMESTAMP
	`, value.PelicanURL, value.PelicanSecretName, value.RouterDomain, generation); err != nil {
		return "", fmt.Errorf("stage pending setup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit pending setup transaction: %w", err)
	}
	return generation, nil
}

// PromotePendingSetup atomically makes the staged candidate active and marks
// setup complete. A caller must activate the candidate runtime before calling
// this method and publish it only after this method succeeds.
func (s *Store) PromotePendingSetup(generation string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin pending setup promotion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var value Settings
	if err := tx.QueryRow(`
		SELECT pelican_url, pelican_secret_name, router_domain
		FROM pending_setup WHERE id = 1 AND generation = ?
	`, generation).Scan(&value.PelicanURL, &value.PelicanSecretName, &value.RouterDomain); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load pending setup: %w", err)
	}
	if err := saveValues(tx, value, true); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM pending_setup WHERE id = 1`); err != nil {
		return fmt.Errorf("clear pending setup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit pending setup promotion: %w", err)
	}
	return nil
}

func (s *Store) save(value Settings, complete bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin settings transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	if err := saveValues(tx, value, complete); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit settings transaction: %w", err)
	}

	return nil
}

func saveValues(writer settingWriter, value Settings, complete bool) error {
	values := []struct {
		key   string
		value string
	}{
		{
			key:   KeyPelicanURL,
			value: value.PelicanURL,
		},
		{
			key:   KeyRouterDomain,
			value: value.RouterDomain,
		},
	}
	if complete {
		values = append(values, struct{ key, value string }{KeySetupCompleted, strconv.FormatBool(true)})
	}
	if value.PelicanSecretName != "" {
		values = append(values, struct{ key, value string }{KeyPelicanSecretName, value.PelicanSecretName})
	} else {
		values = append(values, struct{ key, value string }{KeyPelicanAPIKey, value.PelicanAPIKey})
	}

	for _, setting := range values {
		if err := setValue(writer, setting.key, setting.value); err != nil {
			return fmt.Errorf("save %q: %w", setting.key, err)
		}
	}
	if value.PelicanSecretName != "" {
		if _, err := writer.Exec(`DELETE FROM settings WHERE key = ?`, KeyPelicanAPIKey); err != nil {
			return fmt.Errorf("remove legacy Pelican credential: %w", err)
		}
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
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Settings{}, fmt.Errorf("load %q: %w", KeyPelicanAPIKey, err)
	}
	if err == nil {
		result.PelicanAPIKey = pelicanAPIKey
	}

	pelicanSecretName, err := s.Get(KeyPelicanSecretName)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Settings{}, fmt.Errorf("load %q: %w", KeyPelicanSecretName, err)
	}
	if err == nil {
		result.PelicanSecretName = pelicanSecretName
	}

	routerDomain, err := s.Get(KeyRouterDomain)
	if err != nil {
		return Settings{}, fmt.Errorf("load %q: %w", KeyRouterDomain, err)
	}
	result.RouterDomain = routerDomain

	return result, nil
}

func (s *Store) SaveSetup(value Settings) error {
	return s.save(value, true)
}
