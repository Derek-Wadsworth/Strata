package tracker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/store"
)

func TestSubscribeReceivesVersionCreated(t *testing.T) {
	tr := newTestTracker(t)
	events, unsub := tr.Subscribe(8)
	defer unsub()

	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := tr.Add(path); err != nil {
		t.Fatalf("Add: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Type != EventVersionCreated || ev.VersionNum != 1 {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for version_created event")
	}

	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatalf("WriteFile 2: %v", err)
	}
	if _, err := tr.SnapshotIfChanged(path); err != nil {
		t.Fatalf("SnapshotIfChanged: %v", err)
	}

	select {
	case ev := <-events:
		if ev.Type != EventVersionCreated || ev.VersionNum != 2 {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second event")
	}
}

func TestAPIStyleOpenViaTrackerDeps(t *testing.T) {
	// Smoke: constructing tracker the way api.Open does still works with events.
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
	tr := New(database, snapshots, Options{Debounce: 10 * time.Millisecond})
	ch, unsub := tr.Subscribe(1)
	unsub()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel after unsub")
		}
	default:
		// closed channels still need a receive to observe; try again
		_, ok := <-ch
		if ok {
			t.Fatal("expected closed channel")
		}
	}
}
