package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Derek-Wadsworth/Strata/internal/appdata"
	"github.com/Derek-Wadsworth/Strata/internal/config"
	"github.com/Derek-Wadsworth/Strata/internal/daemon"
	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/graph"
	"github.com/Derek-Wadsworth/Strata/internal/stats"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
	"github.com/Derek-Wadsworth/Strata/internal/watcher"
)

// Client is the core Strata API used by the CLI and (later) the GUI.
// It owns the database connection; call Close when finished.
type Client struct {
	dataDir  string
	cfg      config.Config
	database *db.DB
	tracker  *tracker.Tracker
	log      *slog.Logger
}

// Open loads config, opens the store/db, and returns a ready Client.
func Open(log *slog.Logger) (*Client, error) {
	if log == nil {
		log = slog.Default()
	}
	base, err := appdata.EnsureDir()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(appdata.ConfigPath(base))
	if err != nil {
		return nil, err
	}
	dataDir := base
	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
		dataDir = cfg.DataDir
	}
	database, err := db.Open(appdata.DBPath(dataDir))
	if err != nil {
		return nil, err
	}
	snapshots, err := store.New(dataDir)
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &Client{
		dataDir:  dataDir,
		cfg:      cfg,
		database: database,
		tracker: tracker.New(database, snapshots, tracker.Options{
			Debounce:     cfg.Debounce(),
			MaxFileBytes: cfg.MaxFileBytes,
			Log:          log,
		}),
		log: log,
	}, nil
}

// Close releases database resources.
func (c *Client) Close() error {
	if c.database == nil {
		return nil
	}
	err := c.database.Close()
	c.database = nil
	return err
}

// DataDir returns the active data directory.
func (c *Client) DataDir() string { return c.dataDir }

// LogPath returns the daemon log file path.
func (c *Client) LogPath() string { return appdata.LogPath(c.dataDir) }

// PidPath returns the daemon PID file path.
func (c *Client) PidPath() string { return daemon.PidPath(c.dataDir) }

// AbsPath resolves path to an absolute path.
func (c *Client) AbsPath(path string) (string, error) {
	return filepath.Abs(path)
}

// Add starts tracking a file and records version 1 (or the next version).
func (c *Client) Add(path string) (versionNum int, err error) {
	return c.tracker.Add(path)
}

// List returns active tracked files.
func (c *Client) List() ([]tracker.ListedFile, error) {
	return c.tracker.List()
}

// Remove stops tracking a file but keeps history.
func (c *Client) Remove(path string) error {
	return c.tracker.Remove(path)
}

// History lists versions for a file.
func (c *Client) History(path string) ([]tracker.HistoryEntry, error) {
	return c.tracker.History(path)
}

// Show returns the bytes of a 1-based version.
func (c *Client) Show(path string, versionNum int) ([]byte, error) {
	return c.tracker.Show(path, versionNum)
}

// Diff compares two versions (0,0 = previous vs latest).
func (c *Client) Diff(path string, fromVer, toVer int) (tracker.DiffResult, error) {
	return c.tracker.Diff(path, fromVer, toVer)
}

// Dashboard returns aggregate stats for the GUI.
func (c *Client) Dashboard() (stats.Dashboard, error) {
	return c.tracker.Dashboard()
}

// ListBranches returns branches for a tracked file.
func (c *Client) ListBranches(path string) ([]tracker.BranchInfo, error) {
	return c.tracker.ListBranches(path)
}

// CurrentBranch returns the checked-out branch for a file.
func (c *Client) CurrentBranch(path string) (string, error) {
	return c.tracker.CurrentBranch(path)
}

// CheckoutBranch switches the checked-out branch for a file.
func (c *Client) CheckoutBranch(path, name string) error {
	return c.tracker.CheckoutBranch(path, name)
}

// CreateBranch forks a branch from a version and checks it out.
func (c *Client) CreateBranch(path, name string, fromVersion int) (tracker.BranchInfo, error) {
	return c.tracker.CreateBranch(path, name, fromVersion)
}

// SetVersionTitle sets or clears a version display title.
func (c *Client) SetVersionTitle(path string, versionNum int, title string) error {
	return c.tracker.SetVersionTitle(path, versionNum, title)
}

// Graph returns active files and links between them.
func (c *Client) Graph() (graph.Graph, error) {
	return c.tracker.Graph()
}

// Link relates two tracked files.
func (c *Client) Link(pathA, pathB, note string) (graph.Edge, error) {
	return c.tracker.Link(pathA, pathB, note)
}

// Unlink removes a relationship between two tracked files.
func (c *Client) Unlink(pathA, pathB string) error {
	return c.tracker.Unlink(pathA, pathB)
}

// Neighbors returns files linked to path.
func (c *Client) Neighbors(path string) ([]graph.Node, error) {
	return c.tracker.Neighbors(path)
}

// Restore writes a version back to disk.
func (c *Client) Restore(path string, versionNum int) (tracker.RestoreResult, error) {
	return c.tracker.Restore(path, versionNum)
}

// Export writes a version to dest.
func (c *Client) Export(path string, versionNum int, dest string) error {
	return c.tracker.Export(path, versionNum, dest)
}

// CatchUp records versions for changes made while Strata was off.
func (c *Client) CatchUp() ([]tracker.SnapshotResult, error) {
	return c.tracker.CatchUp()
}

// Run watches tracked files until ctx is cancelled.
func (c *Client) Run(ctx context.Context, onVersion func(tracker.SnapshotResult)) error {
	return c.tracker.Run(ctx, onVersion)
}

// Subscribe receives live EventVersionCreated notifications.
func (c *Client) Subscribe(buffer int) (<-chan tracker.Event, func()) {
	return c.tracker.Subscribe(buffer)
}

// StartDaemon launches the background watcher process.
func (c *Client) StartDaemon() (daemon.Status, error) {
	return daemon.Start(c.dataDir)
}

// StopDaemon stops the background watcher process.
func (c *Client) StopDaemon() error {
	return daemon.Stop(c.dataDir)
}

// DaemonStatus reports whether the background watcher is running.
func (c *Client) DaemonStatus() (daemon.Status, error) {
	return daemon.ReadStatus(c.dataDir)
}

// WriteDaemonPID writes the current process PID (used by the daemon entrypoint).
func (c *Client) WriteDaemonPID(pid int) error {
	return daemon.WritePIDFile(c.dataDir, pid)
}

// WatchFile is a low-level save notifier (Milestone 1 helper).
func (c *Client) WatchFile(ctx context.Context, path string, onChange func(string)) error {
	return watcher.Watch(ctx, path, c.cfg.Debounce(), onChange)
}
