package layout

import (
	"context"
	"path/filepath"
	"testing"

	"xlink/backend/store"
)

func testLayout(t *testing.T) *Service {
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
	return New(db)
}

// SaveSnapshot → GetSnapshot 往返；覆盖写清除旧行。
func TestSnapshotRoundTrip(t *testing.T) {
	s := testLayout(t)
	ctx := context.Background()

	if tabs, err := s.GetSnapshot(ctx); err != nil || len(tabs) != 0 {
		t.Fatalf("empty store: tabs=%v err=%v", tabs, err)
	}

	want := []TabSnapshot{{Position: 1, HostID: 10}, {Position: 2, HostID: 11}, {Position: 3, HostID: 10}}
	if err := s.SaveSnapshot(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("want %d tabs, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tab %d: want %+v got %+v", i, want[i], got[i])
		}
	}

	if err := s.SaveSnapshot(ctx, []TabSnapshot{{Position: 1, HostID: 99}}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetSnapshot(ctx)
	if len(got) != 1 || got[0].HostID != 99 {
		t.Fatalf("overwrite failed: %+v", got)
	}
}

// Save 空快照 = 清空。
func TestSnapshotClear(t *testing.T) {
	s := testLayout(t)
	ctx := context.Background()
	if err := s.SaveSnapshot(ctx, []TabSnapshot{{Position: 1, HostID: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSnapshot(ctx, nil); err != nil {
		t.Fatal(err)
	}
	tabs, _ := s.GetSnapshot(ctx)
	if len(tabs) != 0 {
		t.Fatalf("want empty, got %+v", tabs)
	}
}
