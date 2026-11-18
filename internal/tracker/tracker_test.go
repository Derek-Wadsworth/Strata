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

func TestHistoryShowRestoreExport(t *testing.T) {
	tr := newTestTracker(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := tr.Add(path); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := os.WriteFile(path, []byte("beta"), 0o644); err != nil {
		t.Fatalf("WriteFile beta: %v", err)
	}
	if _, err := tr.SnapshotIfChanged(path); err != nil {
		t.Fatalf("SnapshotIfChanged: %v", err)
	}

	hist, err := tr.History(path)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 2 || hist[0].Number != 1 || hist[1].Number != 2 {
		t.Fatalf("History = %+v", hist)
	}

	got, err := tr.Show(path, 1)
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if string(got) != "alpha" {
		t.Fatalf("Show v1 = %q, want alpha", got)
	}

	dest := filepath.Join(dir, "export-v1.txt")
	if err := tr.Export(path, 1, dest); err != nil {
		t.Fatalf("Export: %v", err)
	}
	exported, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile export: %v", err)
	}
	if string(exported) != "alpha" {
		t.Fatalf("export = %q, want alpha", exported)
	}

	// Unsaved edit on disk; restore should snapshot it first, then write alpha.
	if err := os.WriteFile(path, []byte("gamma"), 0o644); err != nil {
		t.Fatalf("WriteFile gamma: %v", err)
	}
	res, err := tr.Restore(path, 1)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !res.CreatedSafetyNet || res.SafetyVersionNum != 3 {
		t.Fatalf("Restore safety = %+v", res)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after restore: %v", err)
	}
	if string(data) != "alpha" {
		t.Fatalf("file after restore = %q, want alpha", data)
	}

	hist, err = tr.History(path)
	if err != nil {
		t.Fatalf("History after restore: %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("expected 3 versions after safety snapshot, got %d", len(hist))
	}

	// Undo restore by restoring the safety-net version (gamma).
	if _, err := tr.Restore(path, 3); err != nil {
		t.Fatalf("Restore safety version: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after undo: %v", err)
	}
	if string(data) != "gamma" {
		t.Fatalf("file after undo = %q, want gamma", data)
	}
}

func TestCatchUpAndMissing(t *testing.T) {
	tr := newTestTracker(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := tr.Add(path); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := os.WriteFile(path, []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile b: %v", err)
	}

	results, err := tr.CatchUp()
	if err != nil {
		t.Fatalf("CatchUp: %v", err)
	}
	if len(results) != 1 || results[0].Skipped || results[0].VersionNum != 2 {
		t.Fatalf("CatchUp results = %+v", results)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	results, err = tr.CatchUp()
	if err != nil {
		t.Fatalf("CatchUp missing: %v", err)
	}
	if len(results) != 1 || !results[0].Missing {
		t.Fatalf("missing results = %+v", results)
	}
	files, err := tr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 1 || !files[0].Missing {
		t.Fatalf("list missing = %+v", files)
	}
}

func TestSnapshotTooLarge(t *testing.T) {
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
	tr := New(database, snapshots, Options{Debounce: 10 * time.Millisecond, MaxFileBytes: 4})

	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(path, []byte("tiny"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := tr.Add(path); err != nil {
		t.Fatalf("Add small: %v", err)
	}
	if err := os.WriteFile(path, []byte("too-big-now"), 0o644); err != nil {
		t.Fatalf("WriteFile big: %v", err)
	}
	res, err := tr.SnapshotIfChanged(path)
	if err == nil || !res.TooLarge {
		t.Fatalf("want too large, got res=%+v err=%v", res, err)
	}
}

func TestDiffLatestAndExplicit(t *testing.T) {
	tr := newTestTracker(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")

	if err := os.WriteFile(path, []byte("one\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := tr.Add(path); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile 2: %v", err)
	}
	if _, err := tr.SnapshotIfChanged(path); err != nil {
		t.Fatalf("SnapshotIfChanged: %v", err)
	}

	res, err := tr.Diff(path, 0, 0)
	if err != nil {
		t.Fatalf("Diff latest: %v", err)
	}
	if res.FromVer != 1 || res.ToVer != 2 {
		t.Fatalf("versions = %d..%d", res.FromVer, res.ToVer)
	}
	if res.Diff.Binary || res.Diff.LinesAdded != 1 || res.Diff.LinesRemoved != 0 {
		t.Fatalf("diff = %+v", res.Diff)
	}

	hist, err := tr.History(path)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if hist[1].LinesAdded == nil || *hist[1].LinesAdded != 1 {
		t.Fatalf("version 2 line stats = %+v", hist[1])
	}

	res, err = tr.Diff(path, 1, 2)
	if err != nil {
		t.Fatalf("Diff explicit: %v", err)
	}
	if res.Diff.LinesAdded != 1 {
		t.Fatalf("explicit diff stats = %+v", res.Diff)
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
	return New(database, snapshots, Options{Debounce: 10 * time.Millisecond})
}
