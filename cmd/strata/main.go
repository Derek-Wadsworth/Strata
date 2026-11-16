package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"

	"github.com/Derek-Wadsworth/Strata/internal/appdata"
	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
	"github.com/Derek-Wadsworth/Strata/internal/watcher"
)

const version = "v0.0.1"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "strata: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Printf("strata %s\n", version)
		return nil
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Printf("strata %s\n", version)
		return nil
	case "watch":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata watch <file>")
		}
		return watchFile(args[1])
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata add <file>")
		}
		return withTracker(func(t *tracker.Tracker) error {
			n, err := t.Add(args[1])
			if err != nil {
				return err
			}
			abs, _ := filepath.Abs(args[1])
			fmt.Printf("tracking %s (version %d)\n", abs, n)
			return nil
		})
	case "list":
		return withTracker(func(t *tracker.Tracker) error {
			files, err := t.List()
			if err != nil {
				return err
			}
			if len(files) == 0 {
				fmt.Println("no tracked files")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "VERSIONS\tPATH")
			for _, f := range files {
				fmt.Fprintf(w, "%d\t%s\n", f.VersionCount, f.Path)
			}
			return w.Flush()
		})
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata remove <file>")
		}
		return withTracker(func(t *tracker.Tracker) error {
			if err := t.Remove(args[1]); err != nil {
				return err
			}
			abs, _ := filepath.Abs(args[1])
			fmt.Printf("stopped tracking %s (history kept)\n", abs)
			return nil
		})
	case "run":
		return withTracker(func(t *tracker.Tracker) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			files, err := t.List()
			if err != nil {
				return err
			}
			fmt.Printf("watching %d file(s) (Ctrl+C to stop)\n", len(files))
			for _, f := range files {
				fmt.Printf("  %s\n", f.Path)
			}
			return t.Run(ctx, func(res tracker.SnapshotResult) {
				fmt.Printf("version %d: %s\n", res.VersionNum, res.Path)
			})
		})
	default:
		return fmt.Errorf("unknown command %q\n\nusage:\n  strata add <file>\n  strata list\n  strata remove <file>\n  strata run\n  strata watch <file>\n  strata version", args[0])
	}
}

func withTracker(fn func(*tracker.Tracker) error) error {
	dataDir, err := appdata.EnsureDir()
	if err != nil {
		return err
	}
	database, err := db.Open(appdata.DBPath(dataDir))
	if err != nil {
		return err
	}
	defer database.Close()

	snapshots, err := store.New(dataDir)
	if err != nil {
		return err
	}
	return fn(tracker.New(database, snapshots, watcher.DefaultDebounce))
}

func watchFile(path string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("watching %s (Ctrl+C to stop)\n", path)
	return watcher.Watch(ctx, path, watcher.DefaultDebounce, func(changed string) {
		fmt.Printf("saved: %s\n", changed)
	})
}
