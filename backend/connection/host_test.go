package connection

import (
	"context"
	"path/filepath"
	"testing"

	"xlink/backend/store"
)

func testConnServiceWithDB(t *testing.T) *Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(store.DefaultDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := New(nil)
	s.SetStore(db)
	return s
}
