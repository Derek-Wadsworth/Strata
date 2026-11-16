package tracker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/watcher"
)

// Tracker ties the database, snapshot store, and file watcher together.
type Tracker struct {
	db       *db.DB
	store    *store.Store
	debounce time.Duration
}

// New creates a Tracker. The caller owns db lifetime (Close).
func New(database *db.DB, snapshots *store.Store, debounce time.Duration) *Tracker {
	if debounce <= 0 {
		debounce = watcher.DefaultDebounce
	}
	return &Tracker{db: database, store: snapshots, debounce: debounce}
}

// ListedFile is a tracked file with a version count for display.
type ListedFile struct {
	Path         string
	VersionCount int
	AddedAt      time.Time
}

// Add registers path, snapshots it immediately as a new version, and returns
// the version number (1-based count for that file).
func (t *Tracker) Add(path string) (versionNum int, err error) {
	abs, err := absExistingFile(path)
	if err != nil {
		return 0, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return 0, fmt.Errorf("stat file: %w", err)
	}

	fileID, err := t.db.UpsertActiveFile(abs, time.Now())
	if err != nil {
		return 0, err
	}

	hash, err := t.store.Save(abs)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}

	if _, err := t.db.InsertVersion(fileID, hash, info.Size(), time.Now()); err != nil {
		return 0, err
	}

	n, err := t.db.CountVersions(fileID)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// List returns active tracked files.
func (t *Tracker) List() ([]ListedFile, error) {
	files, err := t.db.ListActiveFiles()
	if err != nil {
		return nil, err
	}
	out := make([]ListedFile, 0, len(files))
	for _, f := range files {
		n, err := t.db.CountVersions(f.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, ListedFile{
			Path:         f.Path,
			VersionCount: n,
			AddedAt:      f.AddedAt,
		})
	}
	return out, nil
}

// Remove stops tracking path but keeps its version history.
func (t *Tracker) Remove(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	return t.db.DeactivateFile(abs)
}

// SnapshotResult describes what happened after a save.
type SnapshotResult struct {
	Path       string
	Hash       string
	VersionNum int
	Skipped    bool
}

// SnapshotIfChanged stores a new version when content differs from the latest.
func (t *Tracker) SnapshotIfChanged(path string) (SnapshotResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return SnapshotResult{}, fmt.Errorf("resolve path: %w", err)
	}

	f, err := t.db.GetActiveFileByPath(abs)
	if err != nil {
		return SnapshotResult{}, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return SnapshotResult{}, fmt.Errorf("stat file: %w", err)
	}

	hash, err := t.store.Save(abs)
	if err != nil {
		return SnapshotResult{}, fmt.Errorf("snapshot: %w", err)
	}

	latest, ok, err := t.db.LatestVersion(f.ID)
	if err != nil {
		return SnapshotResult{}, err
	}
	if ok && latest.Hash == hash {
		n, err := t.db.CountVersions(f.ID)
		if err != nil {
			return SnapshotResult{}, err
		}
		return SnapshotResult{Path: abs, Hash: hash, VersionNum: n, Skipped: true}, nil
	}

	if _, err := t.db.InsertVersion(f.ID, hash, info.Size(), time.Now()); err != nil {
		return SnapshotResult{}, err
	}
	n, err := t.db.CountVersions(f.ID)
	if err != nil {
		return SnapshotResult{}, err
	}
	return SnapshotResult{Path: abs, Hash: hash, VersionNum: n, Skipped: false}, nil
}

// Run watches all active files until ctx is cancelled.
// onVersion is called when a new version is recorded (not when skipped).
func (t *Tracker) Run(ctx context.Context, onVersion func(SnapshotResult)) error {
	files, err := t.db.ListActiveFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no tracked files; use: strata add <file>")
	}
	if onVersion == nil {
		onVersion = func(SnapshotResult) {}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, len(files))

	for _, f := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			err := watcher.Watch(ctx, path, t.debounce, func(changed string) {
				res, err := t.SnapshotIfChanged(changed)
				if err != nil {
					fmt.Fprintf(os.Stderr, "strata: snapshot %s: %v\n", changed, err)
					return
				}
				if !res.Skipped {
					onVersion(res)
				}
			})
			if err != nil && ctx.Err() == nil {
				errCh <- err
				cancel()
			}
		}(f.Path)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case err := <-errCh:
			return err
		default:
			return nil
		}
	case err := <-errCh:
		<-done
		return err
	}
}

func absExistingFile(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", abs)
	}
	return abs, nil
}
