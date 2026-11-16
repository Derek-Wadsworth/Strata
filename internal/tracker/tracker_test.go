package tracker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/store"
)

func TestAddListRemoveAndSnapshotIfChanged(t *testing.T) {
	tr := newTestTracker(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	n, err := tr.Add(path)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if n != 1 {
		t.Fatalf("Add version = %d, want 1", n)
	}

	files, err := tr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 1 || files[0].VersionCount != 1 {
		t.Fatalf("List = %+v", files)
	}

	// Unchanged content should skip.
	res, err := tr.SnapshotIfChanged(path)
	if err != nil {
		t.Fatalf("SnapshotIfChanged unchanged: %v", err)
	}
	if !res.Skipped || res.VersionNum != 1 {
		t.Fatalf("unchanged result = %+v", res)
	}

	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatalf("WriteFile v2: %v", err)
	}
	res, err = tr.SnapshotIfChanged(path)
	if err != nil {
		t.Fatalf("SnapshotIfChanged changed: %v", err)
	}
	if res.Skipped || res.VersionNum != 2 {
		t.Fatalf("changed result = %+v", res)
	}

	if err := tr.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	files, err = tr.List()
	if err != nil {
		t.Fatalf("List after remove: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no active files, got %+v", files)
	}
}

func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	root := t.TempDir()
	database, err := db.Open(filepath.Join(root, "strata.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	snapshots, err := store.New(root)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return New(database, snapshots, 10*time.Millisecond)
}
