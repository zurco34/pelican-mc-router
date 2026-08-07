package settings

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
)

func TestNewStore(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}

	if store.db != db {
		t.Fatal("NewStore() did not retain the database connection")
	}
}
func TestStoreSet(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	err = store.Set("router.domain", "mc.example.com")
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	var value string
	err = db.QueryRow(
		`SELECT value FROM settings WHERE key = ?`,
		"router.domain",
	).Scan(&value)
	if err != nil {
		t.Fatalf("query saved setting: %v", err)
	}

	if value != "mc.example.com" {
		t.Fatalf("saved value = %q, want %q", value, "mc.example.com")
	}
}
func TestStoreGet(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	if err := store.Set("router.domain", "mc.example.com"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value, err := store.Get("router.domain")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "mc.example.com" {
		t.Fatalf("Get() = %q, want %q", value, "mc.example.com")
	}
}
func TestStoreGetReturnsErrNotFound(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	_, err = store.Get("missing.setting")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}
func TestStoreSetUpdatesExistingSetting(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	if err := store.Set("router.domain", "old.example.com"); err != nil {
		t.Fatalf("first Set() error = %v", err)
	}

	if err := store.Set("router.domain", "new.example.com"); err != nil {
		t.Fatalf("second Set() error = %v", err)
	}

	value, err := store.Get("router.domain")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "new.example.com" {
		t.Fatalf("Get() = %q, want %q", value, "new.example.com")
	}

	var count int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM settings WHERE key = ?`,
		"router.domain",
	).Scan(&count)
	if err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if count != 1 {
		t.Fatalf("setting row count = %d, want 1", count)
	}
}

func TestStoreIsSetupCompleteDefaultsToFalse(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	complete, err := store.IsSetupComplete()
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}

	if complete {
		t.Fatal("IsSetupComplete() = true, want false")
	}
}

func TestStoreSetSetupComplete(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	if err := store.SetSetupComplete(true); err != nil {
		t.Fatalf("SetSetupComplete() error = %v", err)
	}

	complete, err := store.IsSetupComplete()
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}

	if !complete {
		t.Fatal("IsSetupComplete() = false, want true")
	}
}
func TestStoreIsSetupCompleteRejectsInvalidValue(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	if err := store.Set(KeySetupCompleted, "not-a-boolean"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	_, err = store.IsSetupComplete()
	if err == nil {
		t.Fatal("IsSetupComplete() error = nil, want an error")
	}
}
func TestStoreSaveSettings(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	input := Settings{
		PelicanURL:    "https://panel.example.com",
		PelicanAPIKey: "test-api-key",
		RouterDomain:  "mc.example.com",
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tests := map[string]string{
		KeyPelicanURL:    input.PelicanURL,
		KeyPelicanAPIKey: input.PelicanAPIKey,
		KeyRouterDomain:  input.RouterDomain,
	}

	for key, want := range tests {
		var got string

		err := db.QueryRow(
			`SELECT value FROM settings WHERE key = ?`,
			key,
		).Scan(&got)
		if err != nil {
			t.Fatalf("query setting %q: %v", key, err)
		}

		if got != want {
			t.Fatalf("setting %q = %q, want %q", key, got, want)
		}
	}
}

func TestStoreSaveRollsBackWhenWriteFails(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	original := Settings{
		PelicanURL:    "https://old-panel.example.com",
		PelicanAPIKey: "old-api-key",
		RouterDomain:  "old.mc.example.com",
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("save original settings: %v", err)
	}

	_, err = db.Exec(`
		CREATE TRIGGER fail_router_domain_update
		BEFORE UPDATE ON settings
		WHEN NEW.key = 'router.domain'
		BEGIN
			SELECT RAISE(ABORT, 'forced write failure');
		END
	`)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	updated := Settings{
		PelicanURL:    "https://new-panel.example.com",
		PelicanAPIKey: "new-api-key",
		RouterDomain:  "new.mc.example.com",
	}

	err = store.Save(updated)
	if err == nil {
		t.Fatal("Save() error = nil, want an error")
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got != original {
		t.Fatalf(
			"settings after failed Save() = %#v, want %#v",
			got,
			original,
		)
	}
}

func TestStoreLoadSettings(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	want := Settings{
		PelicanURL:    "https://panel.example.com",
		PelicanAPIKey: "test-api-key",
		RouterDomain:  "mc.example.com",
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
}
func TestStoreLoadReturnsErrNotFoundWhenSettingsAreMissing(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	_, err = store.Load()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want errors.Is(error, ErrNotFound)", err)
	}
}
func TestStoreSaveSetup(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{
		Path: filepath.Join(t.TempDir(), "router.db"),
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store := NewStore(db)

	want := Settings{
		PelicanURL:    "https://panel.example.com/api/application",
		PelicanAPIKey: "test-api-key",
		RouterDomain:  "mc.example.com",
	}

	if err := store.SaveSetup(want); err != nil {
		t.Fatalf("SaveSetup() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}

	complete, err := store.IsSetupComplete()
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}

	if !complete {
		t.Fatal("IsSetupComplete() = false, want true")
	}
}

func TestStorePromotePendingSetup(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)

	pending := Settings{
		PelicanURL:        "https://panel.example.com/api/application",
		PelicanSecretName: "pelican_api_key",
		RouterDomain:      "mc.example.com",
	}
	generation, err := store.StageSetup(pending)
	if err != nil {
		t.Fatalf("StageSetup() error = %v", err)
	}
	complete, err := store.IsSetupComplete()
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}
	if complete {
		t.Fatal("staged setup was marked complete")
	}
	if _, err := store.Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound before promotion", err)
	}

	if err := store.PromotePendingSetup(generation); err != nil {
		t.Fatalf("PromotePendingSetup() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != pending {
		t.Fatalf("Load() = %#v, want %#v", got, pending)
	}
	complete, err = store.IsSetupComplete()
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}
	if !complete {
		t.Fatal("promoted setup was not marked complete")
	}
	var pendingRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_setup`).Scan(&pendingRows); err != nil {
		t.Fatalf("count pending setup rows: %v", err)
	}
	if pendingRows != 0 {
		t.Fatalf("pending setup rows = %d, want 0", pendingRows)
	}
}

func TestStorePromotionRequiresMatchingGeneration(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewStore(db)
	generation, err := store.StageSetup(Settings{PelicanURL: "https://panel.example.test", PelicanSecretName: "credential", RouterDomain: "mc.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PromotePendingSetup("different-generation"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PromotePendingSetup() error = %v, want ErrNotFound", err)
	}
	if complete, err := store.IsSetupComplete(); err != nil || complete {
		t.Fatalf("setup complete = %t, error = %v; want false, nil", complete, err)
	}
	if err := store.PromotePendingSetup(generation); err != nil {
		t.Fatalf("PromotePendingSetup() with matching generation: %v", err)
	}
}

func TestStoreStageSetupRejectsLegacyCredential(t *testing.T) {
	t.Parallel()
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = NewStore(db).StageSetup(Settings{
		PelicanURL:    "https://panel.example.com",
		PelicanAPIKey: "test-api-key",
		RouterDomain:  "mc.example.com",
	})
	if err == nil {
		t.Fatal("StageSetup() error = nil, want an error")
	}
	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pending_setup`).Scan(&rows); err != nil {
		t.Fatalf("count pending setup rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("pending setup rows = %d, want 0", rows)
	}
}

func TestStorePromotionFailureKeepsPendingSetupRetryable(t *testing.T) {
	t.Parallel()

	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	pending := Settings{PelicanURL: "https://panel.example.com", PelicanSecretName: "pelican_api_key", RouterDomain: "mc.example.com"}
	generation, err := store.StageSetup(pending)
	if err != nil {
		t.Fatalf("StageSetup() error = %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER fail_setup_promotion
		BEFORE INSERT ON settings
		WHEN NEW.key = 'setup.completed'
		BEGIN SELECT RAISE(ABORT, 'forced promotion failure'); END
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	if err := store.PromotePendingSetup(generation); err == nil {
		t.Fatal("PromotePendingSetup() error = nil, want an error")
	}
	complete, err := store.IsSetupComplete()
	if err != nil {
		t.Fatalf("IsSetupComplete() error = %v", err)
	}
	if complete {
		t.Fatal("failed promotion marked setup complete")
	}
	var secretName string
	if err := db.QueryRow(`SELECT pelican_secret_name FROM pending_setup WHERE id = 1`).Scan(&secretName); err != nil {
		t.Fatalf("load pending setup: %v", err)
	}
	if secretName != pending.PelicanSecretName {
		t.Fatal("failed promotion did not retain pending setup")
	}
}
