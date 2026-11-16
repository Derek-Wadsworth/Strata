package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOpenMigratesAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	id, err := d.UpsertActiveFile(`/tmp/demo.txt`, time.Now())
	if err != nil {
		t.Fatalf("UpsertActiveFile: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero file id")
	}

	vid, err := d.InsertVersion(id, "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899", 12, time.Now())
	if err != nil {
		t.Fatalf("InsertVersion: %v", err)
	}
	if vid == 0 {
		t.Fatal("expected non-zero version id")
	}

	latest, ok, err := d.LatestVersion(id)
	if err != nil || !ok {
		t.Fatalf("LatestVersion: ok=%v err=%v", ok, err)
	}
	if latest.Size != 12 {
		t.Fatalf("size = %d, want 12", latest.Size)
	}

	files, err := d.ListActiveFiles()
	if err != nil {
		t.Fatalf("ListActiveFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("active files = %d, want 1", len(files))
	}

	if err := d.DeactivateFile(`/tmp/demo.txt`); err != nil {
		t.Fatalf("DeactivateFile: %v", err)
	}
	files, err = d.ListActiveFiles()
	if err != nil {
		t.Fatalf("ListActiveFiles after remove: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("active files after remove = %d, want 0", len(files))
	}

	n, err := d.CountVersions(id)
	if err != nil {
		t.Fatalf("CountVersions: %v", err)
	}
	if n != 1 {
		t.Fatalf("versions kept after remove = %d, want 1", n)
	}
}
