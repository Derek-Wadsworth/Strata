package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/diff"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/watcher"
)

// Sentinel errors for snapshot handling.
var (
	ErrMissing  = errors.New("tracked file is missing")
	ErrTooLarge = errors.New("file exceeds configured size limit")
)

// Options configures tracker behavior.
type Options struct {
	Debounce     time.Duration
	MaxFileBytes int64 // 0 means no limit
	Log          *slog.Logger
}

// Tracker ties the database, snapshot store, and file watcher together.
type Tracker struct {
	db           *db.DB
	store        *store.Store
	debounce     time.Duration
	maxFileBytes int64
	log          *slog.Logger
}

// New creates a Tracker. The caller owns db lifetime (Close).
func New(database *db.DB, snapshots *store.Store, opts Options) *Tracker {
	if opts.Debounce <= 0 {
		opts.Debounce = watcher.DefaultDebounce
	}
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	return &Tracker{
		db:           database,
		store:        snapshots,
		debounce:     opts.Debounce,
		maxFileBytes: opts.MaxFileBytes,
		log:          opts.Log,
	}
}

// ListedFile is a tracked file with a version count for display.
type ListedFile struct {
	Path         string
	VersionCount int
	AddedAt      time.Time
	Missing      bool
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
	if err := t.checkSize(info.Size()); err != nil {
		return 0, err
	}

	fileID, err := t.db.UpsertActiveFile(abs, time.Now())
	if err != nil {
		return 0, err
	}

	hash, err := t.store.Save(abs)
	if err != nil {
		return 0, fmt.Errorf("snapshot: %w", err)
	}

	added, removed := t.lineStatsAgainstLatest(fileID, hash)
	if _, err := t.db.InsertVersion(fileID, hash, info.Size(), time.Now(), added, removed); err != nil {
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
			Missing:      f.Missing,
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
	TooLarge   bool
	Missing    bool
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
		if os.IsNotExist(err) {
			_ = t.db.SetMissing(abs, true)
			return SnapshotResult{Path: abs, Missing: true, Skipped: true}, fmt.Errorf("%w: %s", ErrMissing, abs)
		}
		return SnapshotResult{}, fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return SnapshotResult{}, fmt.Errorf("path is a directory: %s", abs)
	}
	if err := t.checkSize(info.Size()); err != nil {
		t.log.Warn("skipping oversized file", "path", abs, "size", info.Size(), "limit", t.maxFileBytes)
		return SnapshotResult{Path: abs, TooLarge: true, Skipped: true}, err
	}

	hash, err := t.store.Save(abs)
	if err != nil {
		return SnapshotResult{}, fmt.Errorf("snapshot: %w", err)
	}

	if f.Missing {
		_ = t.db.SetMissing(abs, false)
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

	var added, removed *int
	if ok {
		added, removed = t.lineStats(latest.Hash, hash)
	}
	if _, err := t.db.InsertVersion(f.ID, hash, info.Size(), time.Now(), added, removed); err != nil {
		return SnapshotResult{}, err
	}
	n, err := t.db.CountVersions(f.ID)
	if err != nil {
		return SnapshotResult{}, err
	}
	return SnapshotResult{Path: abs, Hash: hash, VersionNum: n, Skipped: false}, nil
}

// CatchUp compares each active file to its latest version and records changes
// that happened while Strata was not running.
func (t *Tracker) CatchUp() ([]SnapshotResult, error) {
	files, err := t.db.ListActiveFiles()
	if err != nil {
		return nil, err
	}
	var out []SnapshotResult
	for _, f := range files {
		res, err := t.SnapshotIfChanged(f.Path)
		if err != nil {
			if errors.Is(err, ErrMissing) || errors.Is(err, ErrTooLarge) {
				t.log.Info("catch-up skipped", "path", f.Path, "err", err)
				out = append(out, res)
				continue
			}
			return out, err
		}
		if !res.Skipped {
			t.log.Info("catch-up recorded version", "path", res.Path, "version", res.VersionNum)
		}
		out = append(out, res)
	}
	return out, nil
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
	for _, f := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			// Parent dir may be missing; retry is out of scope — mark missing and exit this watch.
			if _, err := os.Stat(filepath.Dir(path)); err != nil {
				t.log.Warn("parent directory unavailable", "path", path, "err", err)
				_ = t.db.SetMissing(path, true)
				return
			}
			err := watcher.Watch(ctx, path, t.debounce, func(changed string) {
				res, err := t.SnapshotIfChanged(changed)
				if err != nil {
					if errors.Is(err, ErrMissing) || errors.Is(err, ErrTooLarge) {
						return
					}
					t.log.Error("snapshot failed", "path", changed, "err", err)
					return
				}
				if !res.Skipped {
					onVersion(res)
				}
			})
			if err != nil && ctx.Err() == nil {
				t.log.Warn("watcher stopped", "path", path, "err", err)
				_ = t.db.SetMissing(path, true)
			}
		}(f.Path)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		<-done
		return nil
	case <-done:
		return nil
	}
}

func (t *Tracker) checkSize(size int64) error {
	if t.maxFileBytes > 0 && size > t.maxFileBytes {
		return fmt.Errorf("%w (%d > %d)", ErrTooLarge, size, t.maxFileBytes)
	}
	return nil
}

// HistoryEntry is one version in a file's timeline.
type HistoryEntry struct {
	Number       int
	CreatedAt    time.Time
	Size         int64
	Hash         string
	LinesAdded   *int
	LinesRemoved *int
}

// History lists versions for path, oldest first as version 1, 2, ...
func (t *Tracker) History(path string) ([]HistoryEntry, error) {
	f, err := t.fileByPath(path)
	if err != nil {
		return nil, err
	}
	versions, err := t.db.ListVersions(f.ID)
	if err != nil {
		return nil, err
	}
	out := make([]HistoryEntry, 0, len(versions))
	for i, v := range versions {
		out = append(out, HistoryEntry{
			Number:       i + 1,
			CreatedAt:    v.CreatedAt,
			Size:         v.Size,
			Hash:         v.Hash,
			LinesAdded:   v.LinesAdded,
			LinesRemoved: v.LinesRemoved,
		})
	}
	return out, nil
}

// DiffResult is a compare between two versions of a tracked file.
type DiffResult struct {
	Path    string
	FromVer int
	ToVer   int
	Diff    diff.Result
}

// Diff compares two versions of path.
// If fromVer and toVer are both 0, compares previous vs latest.
func (t *Tracker) Diff(path string, fromVer, toVer int) (DiffResult, error) {
	f, err := t.fileByPath(path)
	if err != nil {
		return DiffResult{}, err
	}

	n, err := t.db.CountVersions(f.ID)
	if err != nil {
		return DiffResult{}, err
	}
	if fromVer == 0 && toVer == 0 {
		if n < 2 {
			return DiffResult{}, fmt.Errorf("need at least 2 versions to diff; got %d", n)
		}
		fromVer, toVer = n-1, n
	} else if fromVer == 0 || toVer == 0 {
		return DiffResult{}, fmt.Errorf("both versions are required (got %d and %d)", fromVer, toVer)
	}

	oldVer, err := t.db.VersionByNumber(f.ID, fromVer)
	if err != nil {
		return DiffResult{}, err
	}
	newVer, err := t.db.VersionByNumber(f.ID, toVer)
	if err != nil {
		return DiffResult{}, err
	}

	oldData, err := t.store.Load(oldVer.Hash)
	if err != nil {
		return DiffResult{}, fmt.Errorf("load version %d: %w", fromVer, err)
	}
	newData, err := t.store.Load(newVer.Hash)
	if err != nil {
		return DiffResult{}, fmt.Errorf("load version %d: %w", toVer, err)
	}

	return DiffResult{
		Path:    f.Path,
		FromVer: fromVer,
		ToVer:   toVer,
		Diff:    diff.Compare(oldData, newData),
	}, nil
}

func (t *Tracker) lineStatsAgainstLatest(fileID int64, newHash string) (added, removed *int) {
	latest, ok, err := t.db.LatestVersion(fileID)
	if err != nil || !ok {
		return nil, nil
	}
	return t.lineStats(latest.Hash, newHash)
}

func (t *Tracker) lineStats(oldHash, newHash string) (added, removed *int) {
	oldData, err := t.store.Load(oldHash)
	if err != nil {
		return nil, nil
	}
	newData, err := t.store.Load(newHash)
	if err != nil {
		return nil, nil
	}
	res := diff.Compare(oldData, newData)
	if res.Binary {
		return nil, nil
	}
	a, r := res.LinesAdded, res.LinesRemoved
	return &a, &r
}

// Show returns the content of a 1-based version.
func (t *Tracker) Show(path string, versionNum int) ([]byte, error) {
	f, err := t.fileByPath(path)
	if err != nil {
		return nil, err
	}
	v, err := t.db.VersionByNumber(f.ID, versionNum)
	if err != nil {
		return nil, err
	}
	data, err := t.store.Load(v.Hash)
	if err != nil {
		return nil, fmt.Errorf("load version %d: %w", versionNum, err)
	}
	return data, nil
}

// RestoreResult describes a successful restore.
type RestoreResult struct {
	Path             string
	RestoredVersion  int
	SafetyVersionNum int  // new version of pre-restore content, if one was created
	CreatedSafetyNet bool // true when current content was snapshotted before overwrite
}

// Restore writes versionNum back to path. Current on-disk content is snapshotted
// first (when it differs) so the restore itself is undoable.
func (t *Tracker) Restore(path string, versionNum int) (RestoreResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("resolve path: %w", err)
	}
	f, err := t.db.GetFileByPath(abs)
	if err != nil {
		return RestoreResult{}, err
	}

	v, err := t.db.VersionByNumber(f.ID, versionNum)
	if err != nil {
		return RestoreResult{}, err
	}

	result := RestoreResult{Path: abs, RestoredVersion: versionNum}
	if _, statErr := os.Stat(abs); statErr == nil && f.Active {
		pre, err := t.SnapshotIfChanged(abs)
		if err != nil {
			return RestoreResult{}, fmt.Errorf("snapshot before restore: %w", err)
		}
		if !pre.Skipped {
			result.CreatedSafetyNet = true
			result.SafetyVersionNum = pre.VersionNum
		}
	}

	data, err := t.store.Load(v.Hash)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("load version %d: %w", versionNum, err)
	}
	if err := writeFileAtomic(abs, data); err != nil {
		return RestoreResult{}, err
	}
	return result, nil
}

// Export writes versionNum's content to dest (does not modify the tracked file).
func (t *Tracker) Export(path string, versionNum int, dest string) error {
	data, err := t.Show(path, versionNum)
	if err != nil {
		return err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve dest: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absDest), 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}
	return writeFileAtomic(absDest, data)
}

func (t *Tracker) fileByPath(path string) (db.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return db.File{}, fmt.Errorf("resolve path: %w", err)
	}
	return t.db.GetFileByPath(abs)
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

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".strata-restore-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}
