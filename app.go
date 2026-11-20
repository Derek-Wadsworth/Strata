package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/api"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails backend bound to the frontend.
type App struct {
	ctx    context.Context
	client *api.Client
	cancel context.CancelFunc
	unsub  func()
}

func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	client, err := api.Open(nil)
	if err != nil {
		runtime.LogError(ctx, "open api: "+err.Error())
		return
	}
	a.client = client

	runCtx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel

	events, unsub := client.Subscribe(64)
	a.unsub = unsub
	go func() {
		for ev := range events {
			if ev.Type == tracker.EventVersionCreated {
				runtime.EventsEmit(a.ctx, "version:created", map[string]any{
					"path":       ev.Path,
					"versionNum": ev.VersionNum,
				})
			}
		}
	}()

	go func() {
		if _, err := client.CatchUp(); err != nil {
			runtime.LogError(a.ctx, "catch-up: "+err.Error())
		}
		_ = client.Run(runCtx, nil)
	}()
}

// shutdown cleans up resources.
func (a *App) shutdown(ctx context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
	if a.unsub != nil {
		a.unsub()
	}
	if a.client != nil {
		_ = a.client.Close()
	}
}

// FileInfo is a tracked file for the sidebar.
type FileInfo struct {
	Path         string `json:"path"`
	VersionCount int    `json:"versionCount"`
	Missing      bool   `json:"missing"`
	AddedAt      string `json:"addedAt"`
}

// VersionInfo is one row in the version timeline.
type VersionInfo struct {
	Number       int    `json:"number"`
	CreatedAt    string `json:"createdAt"`
	Size         int64  `json:"size"`
	LinesAdded   *int   `json:"linesAdded"`
	LinesRemoved *int   `json:"linesRemoved"`
}

// RestoreInfo is returned after a restore.
type RestoreInfo struct {
	Path             string `json:"path"`
	RestoredVersion  int    `json:"restoredVersion"`
	CreatedSafetyNet bool   `json:"createdSafetyNet"`
	SafetyVersionNum int    `json:"safetyVersionNum"`
}

// ListFiles returns active tracked files.
func (a *App) ListFiles() ([]FileInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("app not ready")
	}
	files, err := a.client.List()
	if err != nil {
		return nil, err
	}
	out := make([]FileInfo, 0, len(files))
	for _, f := range files {
		out = append(out, FileInfo{
			Path:         f.Path,
			VersionCount: f.VersionCount,
			Missing:      f.Missing,
			AddedAt:      f.AddedAt.Local().Format(time.RFC3339),
		})
	}
	return out, nil
}

// AddFile opens a native file picker and starts tracking the chosen file.
func (a *App) AddFile() (FileInfo, error) {
	if a.client == nil {
		return FileInfo{}, fmt.Errorf("app not ready")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Track a file with Strata",
	})
	if err != nil {
		return FileInfo{}, err
	}
	if path == "" {
		return FileInfo{}, fmt.Errorf("cancelled")
	}
	n, err := a.client.Add(path)
	if err != nil {
		return FileInfo{}, err
	}
	abs, _ := a.client.AbsPath(path)
	return FileInfo{Path: abs, VersionCount: n, Missing: false}, nil
}

// RemoveFile stops tracking a file (history kept).
func (a *App) RemoveFile(path string) error {
	if a.client == nil {
		return fmt.Errorf("app not ready")
	}
	return a.client.Remove(path)
}

// History returns the version timeline for a file.
func (a *App) History(path string) ([]VersionInfo, error) {
	if a.client == nil {
		return nil, fmt.Errorf("app not ready")
	}
	entries, err := a.client.History(path)
	if err != nil {
		return nil, err
	}
	out := make([]VersionInfo, 0, len(entries))
	for _, e := range entries {
		out = append(out, VersionInfo{
			Number:       e.Number,
			CreatedAt:    e.CreatedAt.Local().Format(time.RFC3339),
			Size:         e.Size,
			LinesAdded:   e.LinesAdded,
			LinesRemoved: e.LinesRemoved,
		})
	}
	return out, nil
}

// ShowVersion returns a version's content as text.
func (a *App) ShowVersion(path string, version int) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("app not ready")
	}
	data, err := a.client.Show(path, version)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RestoreVersion restores a version to disk (with safety snapshot when needed).
func (a *App) RestoreVersion(path string, version int) (RestoreInfo, error) {
	if a.client == nil {
		return RestoreInfo{}, fmt.Errorf("app not ready")
	}
	res, err := a.client.Restore(path, version)
	if err != nil {
		return RestoreInfo{}, err
	}
	return RestoreInfo{
		Path:             res.Path,
		RestoredVersion:  res.RestoredVersion,
		CreatedSafetyNet: res.CreatedSafetyNet,
		SafetyVersionNum: res.SafetyVersionNum,
	}, nil
}
