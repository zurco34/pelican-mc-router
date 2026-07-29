package actionhistory

import (
	"context"
	"errors"
	"github.com/zurco34/pelican-mc-router/internal/storage/sqlite"
	"path/filepath"
	"testing"
)

func TestStoreRejectsUnboundedValues(t *testing.T) {
	db, err := sqlite.Open(sqlite.Config{Path: filepath.Join(t.TempDir(), "router.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	err = NewStore(db).Append(context.Background(), Event{Action: "secret-value", Outcome: OutcomeSuccess})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Append() error = %v", err)
	}
}
