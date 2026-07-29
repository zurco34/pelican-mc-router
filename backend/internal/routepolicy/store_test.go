package routepolicy

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
)

func TestStoreCreateGetAndList(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, Policy{
		ServerUUID:      "3f2504e0-4f89-11d3-9a0c-0305e82c3301",
		PrimaryHostname: "survival.mc.example.com",
		Aliases:         []string{"play.mc.example.com", "survival-alt.mc.example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Revision != 1 {
		t.Fatalf("created revision = %d, want 1", created.Revision)
	}
	if !reflect.DeepEqual(created.Aliases, []string{"play.mc.example.com", "survival-alt.mc.example.com"}) {
		t.Fatalf("created aliases = %#v", created.Aliases)
	}

	got, err := store.Get(ctx, created.ServerUUID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !reflect.DeepEqual(got, created) {
		t.Fatalf("Get() = %#v, want %#v", got, created)
	}

	second, err := store.Create(ctx, Policy{ServerUUID: "a-server-uuid", Excluded: true})
	if err != nil {
		t.Fatalf("create second policy: %v", err)
	}
	policies, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if !reflect.DeepEqual(policies, []Policy{created, second}) {
		t.Fatalf("List() = %#v, want %#v", policies, []Policy{created, second})
	}
}

func TestStoreUpdateUsesRevisionAndReplacesAliases(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()
	created, err := store.Create(ctx, Policy{
		ServerUUID: "server-uuid",
		Aliases:    []string{"old.mc.example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := store.Update(ctx, Policy{
		ServerUUID:      created.ServerUUID,
		PrimaryHostname: "primary.mc.example.com",
		Aliases:         []string{"new.mc.example.com"},
		Excluded:        true,
	}, created.Revision)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	want := Policy{
		ServerUUID:      created.ServerUUID,
		PrimaryHostname: "primary.mc.example.com",
		Aliases:         []string{"new.mc.example.com"},
		Excluded:        true,
		Revision:        2,
	}
	if !reflect.DeepEqual(updated, want) {
		t.Fatalf("Update() = %#v, want %#v", updated, want)
	}

	_, err = store.Update(ctx, updated, created.Revision)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Update() error = %v, want ErrConflict", err)
	}
}

func TestStoreDeleteUsesRevisionAndRemovesAliases(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()
	created, err := store.Create(ctx, Policy{
		ServerUUID: "server-uuid",
		Aliases:    []string{"alias.mc.example.com"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.Delete(ctx, created.ServerUUID, created.Revision+1); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale Delete() error = %v, want ErrConflict", err)
	}
	if err := store.Delete(ctx, created.ServerUUID, created.Revision); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, created.ServerUUID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Delete() error = %v, want ErrNotFound", err)
	}
	var aliases int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM route_policy_aliases`).Scan(&aliases); err != nil {
		t.Fatalf("count aliases: %v", err)
	}
	if aliases != 0 {
		t.Fatalf("alias count = %d, want 0", aliases)
	}
}

func TestStoreRejectsInvalidPolicies(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	tests := []struct {
		name   string
		policy Policy
	}{
		{name: "empty UUID", policy: Policy{}},
		{name: "empty alias", policy: Policy{ServerUUID: "server", Aliases: []string{""}}},
		{name: "duplicate alias", policy: Policy{ServerUUID: "server", Aliases: []string{"a", "a"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := store.Create(ctx, test.policy); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestStoreMissingPolicies(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if _, err := store.Update(ctx, Policy{ServerUUID: "missing"}, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update() error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(ctx, "missing", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() error = %v, want ErrNotFound", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewStore(db)
}
