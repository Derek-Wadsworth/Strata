package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store is a content-addressed snapshot store. Objects live under
// <root>/objects/ab/cdef... named by the SHA-256 of their contents.
type Store struct {
	root string
}

// New creates a Store rooted at root and ensures the objects directory exists.
func New(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("store root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve store root: %w", err)
	}
	objects := filepath.Join(abs, "objects")
	if err := os.MkdirAll(objects, 0o755); err != nil {
		return nil, fmt.Errorf("create objects dir: %w", err)
	}
	return &Store{root: abs}, nil
}

// Save reads the file at path, stores it by SHA-256 content hash if needed,
// and returns the hex hash. Identical content is stored only once.
func (s *Store) Save(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	hash := hex.EncodeToString(h.Sum(nil))

	if s.Exists(hash) {
		return hash, nil
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind file: %w", err)
	}
	if err := s.writeObject(hash, f); err != nil {
		return "", err
	}
	return hash, nil
}

// Load returns the bytes stored under hash.
func (s *Store) Load(hash string) ([]byte, error) {
	if err := validateHash(hash); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.objectPath(hash))
	if err != nil {
		return nil, fmt.Errorf("load object %s: %w", hash, err)
	}
	return data, nil
}

// Exists reports whether an object with the given hash is already stored.
func (s *Store) Exists(hash string) bool {
	if err := validateHash(hash); err != nil {
		return false
	}
	_, err := os.Stat(s.objectPath(hash))
	return err == nil
}

func (s *Store) objectPath(hash string) string {
	return filepath.Join(s.root, "objects", hash[:2], hash[2:])
}

func (s *Store) writeObject(hash string, r io.Reader) error {
	dir := filepath.Join(s.root, "objects", hash[:2])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create object dir: %w", err)
	}

	dest := s.objectPath(hash)
	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return fmt.Errorf("create temp object: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName) // no-op if rename succeeded
	}()

	if _, err := io.Copy(tmp, r); err != nil {
		return fmt.Errorf("write object: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp object: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		// Another Save may have won the race; treat existing object as success.
		if s.Exists(hash) {
			return nil
		}
		return fmt.Errorf("finalize object: %w", err)
	}
	return nil
}

func validateHash(hash string) error {
	if len(hash) != sha256.Size*2 {
		return fmt.Errorf("invalid hash length: %d", len(hash))
	}
	for _, c := range hash {
		if !isHex(c) {
			return fmt.Errorf("invalid hash %q", hash)
		}
	}
	// Reject path tricks like ".." even though hex check usually covers it.
	if strings.Contains(hash, string(os.PathSeparator)) || strings.Contains(hash, "/") || strings.Contains(hash, `\`) {
		return fmt.Errorf("invalid hash %q", hash)
	}
	return nil
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
