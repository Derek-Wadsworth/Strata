package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	a, err := d.UpsertActiveFile(`/tmp/a.md`, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.UpsertActiveFile(`/tmp/b.go`, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := d.UpsertActiveFile(`/tmp/c.txt`, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	link, err := d.LinkFiles(b, a, " design notes ")
	if err != nil {
		t.Fatalf("LinkFiles: %v", err)
	}
	if link.AID != a || link.BID != b {
		t.Fatalf("canonical ids = %d,%d want %d,%d", link.AID, link.BID, a, b)
	}
	if link.Note != "design notes" {
		t.Fatalf("note = %q", link.Note)
	}

	if _, err := d.LinkFiles(a, b, ""); err == nil {
		t.Fatal("expected duplicate link error")
	}
	if _, err := d.LinkFiles(a, a, ""); err == nil {
		t.Fatal("expected self-link error")
	}

	if _, err := d.LinkFiles(a, c, ""); err != nil {
		t.Fatalf("LinkFiles a-c: %v", err)
	}
	all, err := d.ListActiveLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("links = %d", len(all))
	}

	forFile, err := d.ListLinksForFile(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(forFile) != 2 {
		t.Fatalf("links for a = %d", len(forFile))
	}

	if err := d.UnlinkFiles(b, a); err != nil {
		t.Fatalf("UnlinkFiles: %v", err)
	}
	all, _ = d.ListActiveLinks()
	if len(all) != 1 {
		t.Fatalf("after unlink = %d", len(all))
	}
}
