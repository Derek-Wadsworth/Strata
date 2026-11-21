package tracker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
)

func TestBranchCheckoutRecordsNewVersions(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(file, []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	database, err := db.Open(filepath.Join(dir, "strata.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	snapshots, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	tr := tracker.New(database, snapshots, tracker.Options{})

	if _, err := tr.Add(file); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := os.WriteFile(file, []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.SnapshotIfChanged(file); err != nil {
		t.Fatalf("Snapshot 2: %v", err)
	}

	b, err := tr.CreateBranch(file, "experiment", 2)
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if b.Name != "experiment" || !b.Current {
		t.Fatalf("branch = %+v", b)
	}

	if err := os.WriteFile(file, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := tr.SnapshotIfChanged(file)
	if err != nil || res.Skipped {
		t.Fatalf("Snapshot 3: %+v err=%v", res, err)
	}

	hist, err := tr.History(file)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 3 {
		t.Fatalf("versions = %d", len(hist))
	}
	if hist[0].Branch != "main" || hist[1].Branch != "main" {
		t.Fatalf("early branches = %q %q", hist[0].Branch, hist[1].Branch)
	}
	if hist[2].Branch != "experiment" {
		t.Fatalf("v3 branch = %q", hist[2].Branch)
	}

	if err := tr.SetVersionTitle(file, 3, "add third line"); err != nil {
		t.Fatalf("SetVersionTitle: %v", err)
	}
	hist, _ = tr.History(file)
	if hist[2].Title != "add third line" {
		t.Fatalf("title = %q", hist[2].Title)
	}

	if err := tr.CheckoutBranch(file, "main"); err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}
	cur, err := tr.CurrentBranch(file)
	if err != nil || cur != "main" {
		t.Fatalf("current = %q err=%v", cur, err)
	}
}
