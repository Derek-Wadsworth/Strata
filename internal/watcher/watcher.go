package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const DefaultDebounce = 300 * time.Millisecond

// Watch monitors path for saves and calls onChange once per debounced change.
// It watches the parent directory so atomic editor saves (temp + rename) are seen.
// Watch blocks until ctx is cancelled.
func Watch(ctx context.Context, path string, debounce time.Duration, onChange func(path string)) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if onChange == nil {
		return fmt.Errorf("onChange is required")
	}
	if debounce <= 0 {
		debounce = DefaultDebounce
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(filepath.Dir(abs)); err != nil {
		return fmt.Errorf("parent directory: %w", err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer w.Close()

	dir := filepath.Dir(abs)
	target := filepath.Base(abs)
	if err := w.Add(dir); err != nil {
		return fmt.Errorf("watch %s: %w", dir, err)
	}

	timer := time.NewTimer(debounce)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	pending := false
	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher: %w", err)
		case event, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !isTargetEvent(event, target) {
				continue
			}
			pending = true
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
		case <-timer.C:
			if !pending {
				continue
			}
			pending = false
			onChange(abs)
		}
	}
}

func isTargetEvent(event fsnotify.Event, targetBase string) bool {
	if filepath.Base(event.Name) != targetBase {
		return false
	}
	// Editors typically surface saves as Write, Create, and/or Rename.
	return event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
}
