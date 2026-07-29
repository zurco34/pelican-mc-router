package routepolicy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Store owns durable route-policy records.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, policy Policy) (Policy, error) {
	if err := validate(policy); err != nil {
		return Policy{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Policy{}, fmt.Errorf("begin route policy create: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO route_policies (server_uuid, primary_hostname, excluded)
		VALUES (?, ?, ?)
	`, policy.ServerUUID, nullableString(policy.PrimaryHostname), policy.Excluded); err != nil {
		return Policy{}, fmt.Errorf("insert route policy: %w", err)
	}
	if err := replaceAliases(ctx, tx, policy.ServerUUID, policy.Aliases); err != nil {
		return Policy{}, err
	}
	if err := tx.Commit(); err != nil {
		return Policy{}, fmt.Errorf("commit route policy create: %w", err)
	}

	return s.Get(ctx, policy.ServerUUID)
}

func (s *Store) Get(ctx context.Context, serverUUID string) (Policy, error) {
	if strings.TrimSpace(serverUUID) == "" {
		return Policy{}, fmt.Errorf("get route policy: %w", ErrInvalid)
	}

	var policy Policy
	var primaryHostname sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT server_uuid, primary_hostname, excluded, revision
		FROM route_policies
		WHERE server_uuid = ?
	`, serverUUID).Scan(
		&policy.ServerUUID,
		&primaryHostname,
		&policy.Excluded,
		&policy.Revision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Policy{}, ErrNotFound
	}
	if err != nil {
		return Policy{}, fmt.Errorf("get route policy: %w", err)
	}
	if primaryHostname.Valid {
		policy.PrimaryHostname = primaryHostname.String
	}

	aliases, err := s.aliases(ctx, serverUUID)
	if err != nil {
		return Policy{}, err
	}
	policy.Aliases = aliases
	return policy, nil
}

func (s *Store) List(ctx context.Context) ([]Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT server_uuid, primary_hostname, excluded, revision
		FROM route_policies
		ORDER BY server_uuid
	`)
	if err != nil {
		return nil, fmt.Errorf("list route policies: %w", err)
	}
	defer rows.Close()

	policies := make([]Policy, 0)
	for rows.Next() {
		var policy Policy
		var primaryHostname sql.NullString
		if err := rows.Scan(
			&policy.ServerUUID,
			&primaryHostname,
			&policy.Excluded,
			&policy.Revision,
		); err != nil {
			return nil, fmt.Errorf("scan route policy: %w", err)
		}
		if primaryHostname.Valid {
			policy.PrimaryHostname = primaryHostname.String
		}
		aliases, err := s.aliases(ctx, policy.ServerUUID)
		if err != nil {
			return nil, err
		}
		policy.Aliases = aliases
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate route policies: %w", err)
	}
	return policies, nil
}

func (s *Store) Update(ctx context.Context, policy Policy, expectedRevision int64) (Policy, error) {
	if err := validate(policy); err != nil {
		return Policy{}, err
	}
	if expectedRevision <= 0 {
		return Policy{}, fmt.Errorf("update route policy: %w", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Policy{}, fmt.Errorf("begin route policy update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE route_policies
		SET primary_hostname = ?, excluded = ?, revision = revision + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE server_uuid = ? AND revision = ?
	`, nullableString(policy.PrimaryHostname), policy.Excluded, policy.ServerUUID, expectedRevision)
	if err != nil {
		return Policy{}, fmt.Errorf("update route policy: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return Policy{}, fmt.Errorf("check route policy update: %w", err)
	}
	if updated == 0 {
		exists, err := policyExists(ctx, tx, policy.ServerUUID)
		if err != nil {
			return Policy{}, err
		}
		if !exists {
			return Policy{}, ErrNotFound
		}
		return Policy{}, ErrConflict
	}
	if err := replaceAliases(ctx, tx, policy.ServerUUID, policy.Aliases); err != nil {
		return Policy{}, err
	}
	if err := tx.Commit(); err != nil {
		return Policy{}, fmt.Errorf("commit route policy update: %w", err)
	}

	return s.Get(ctx, policy.ServerUUID)
}

func (s *Store) Delete(ctx context.Context, serverUUID string, expectedRevision int64) error {
	if strings.TrimSpace(serverUUID) == "" || expectedRevision <= 0 {
		return fmt.Errorf("delete route policy: %w", ErrInvalid)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin route policy delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var revision int64
	err = tx.QueryRowContext(ctx, `
		SELECT revision FROM route_policies WHERE server_uuid = ?
	`, serverUUID).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get route policy for delete: %w", err)
	}
	if revision != expectedRevision {
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM route_policy_aliases WHERE server_uuid = ?
	`, serverUUID); err != nil {
		return fmt.Errorf("remove route policy aliases: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM route_policies WHERE server_uuid = ?
	`, serverUUID); err != nil {
		return fmt.Errorf("delete route policy: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit route policy delete: %w", err)
	}
	return nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func policyExists(ctx context.Context, query queryer, serverUUID string) (bool, error) {
	var found int
	if err := query.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM route_policies WHERE server_uuid = ?)
	`, serverUUID).Scan(&found); err != nil {
		return false, fmt.Errorf("check route policy existence: %w", err)
	}
	return found == 1, nil
}

func (s *Store) aliases(ctx context.Context, serverUUID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT hostname FROM route_policy_aliases
		WHERE server_uuid = ? ORDER BY hostname
	`, serverUUID)
	if err != nil {
		return nil, fmt.Errorf("list route policy aliases: %w", err)
	}
	defer rows.Close()

	aliases := make([]string, 0)
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, fmt.Errorf("scan route policy alias: %w", err)
		}
		aliases = append(aliases, alias)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate route policy aliases: %w", err)
	}
	return aliases, nil
}

func replaceAliases(ctx context.Context, tx *sql.Tx, serverUUID string, aliases []string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM route_policy_aliases WHERE server_uuid = ?
	`, serverUUID); err != nil {
		return fmt.Errorf("remove route policy aliases: %w", err)
	}
	for _, alias := range aliases {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO route_policy_aliases (server_uuid, hostname) VALUES (?, ?)
		`, serverUUID, alias); err != nil {
			return fmt.Errorf("insert route policy alias: %w", err)
		}
	}
	return nil
}

func validate(policy Policy) error {
	if strings.TrimSpace(policy.ServerUUID) == "" {
		return fmt.Errorf("route policy server UUID: %w", ErrInvalid)
	}

	aliases := make(map[string]struct{}, len(policy.Aliases))
	for _, alias := range policy.Aliases {
		if strings.TrimSpace(alias) == "" {
			return fmt.Errorf("route policy alias: %w", ErrInvalid)
		}
		if _, exists := aliases[alias]; exists {
			return fmt.Errorf("duplicate route policy alias: %w", ErrInvalid)
		}
		aliases[alias] = struct{}{}
	}
	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
