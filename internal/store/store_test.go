package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	s := newTestStore(t)
	path := writeTempFile(t, "hello strata")

	hash, err := s.Save(path)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if hash == "" {
		t.Fatal("Save returned empty hash")
	}

	got, err := s.Load(hash)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != "hello strata" {
		t.Fatalf("Load = %q, want %q", got, "hello strata")
	}
}

func TestSaveDeduplicatesIdenticalContent(t *testing.T) {
	s := newTestStore(t)
	content := []byte("same bytes twice")

	path1 := writeTempFile(t, string(content))
	path2 := writeTempFile(t, string(content))

	hash1, err := s.Save(path1)
	if err != nil {
		t.Fatalf("Save first: %v", err)
	}
	hash2, err := s.Save(path2)
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("hashes differ: %s vs %s", hash1, hash2)
	}

	objectsDir := filepath.Join(s.root, "objects")
	count := 0
	err = filepath.WalkDir(objectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk objects: %v", err)
	}
	if count != 1 {
		t.Fatalf("object files = %d, want 1", count)
	}
	if !s.Exists(hash1) {
		t.Fatal("Exists returned false for saved hash")
	}
}

func TestSaveHashMatchesSHA256(t *testing.T) {
	s := newTestStore(t)
	content := "fingerprint me"
	path := writeTempFile(t, content)

	hash, err := s.Save(path)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	sum := sha256.Sum256([]byte(content))
	want := hex.EncodeToString(sum[:])
	if hash != want {
		t.Fatalf("hash = %s, want %s", hash, want)
	}
}

func TestLoadMissingObject(t *testing.T) {
	s := newTestStore(t)
	missing := hex.EncodeToString(make([]byte, sha256.Size))
	_, err := s.Load(missing)
	if err == nil {
		t.Fatal("Load missing object: want error")
	}
}

func TestExistsInvalidHash(t *testing.T) {
	s := newTestStore(t)
	if s.Exists("not-a-hash") {
		t.Fatal("Exists(invalid) = true, want false")
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
