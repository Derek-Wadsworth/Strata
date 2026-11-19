package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenAddListSubscribe(t *testing.T) {
	// Isolate app data so we do not touch the developer's real Strata dir.
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp) // Windows UserConfigDir
	t.Setenv("HOME", tmp)    // non-Windows fallback used by appdata
	t.Setenv("XDG_CONFIG_HOME", tmp)

	c, err := Open(nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()

	events, unsub := c.Subscribe(4)
	defer unsub()

	path := filepath.Join(tmp, "demo.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	n, err := c.Add(path)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if n != 1 {
		t.Fatalf("version = %d", n)
	}
	files, err := c.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %+v", files)
	}

	select {
	case ev := <-events:
		if ev.VersionNum != 1 {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for version_created event")
	}
}
