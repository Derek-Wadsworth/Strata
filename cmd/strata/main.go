package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

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
	case "history":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata history <file>")
		}
		return withTracker(func(t *tracker.Tracker) error {
			entries, err := t.History(args[1])
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("no versions")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "VERSION\tTIME\tSIZE")
			for _, e := range entries {
				fmt.Fprintf(w, "%d\t%s\t%d\n",
					e.Number,
					e.CreatedAt.Local().Format(time.RFC3339),
					e.Size,
				)
			}
			return w.Flush()
		})
	case "show":
		if len(args) != 3 {
			return fmt.Errorf("usage: strata show <file> <version>")
		}
		num, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid version %q", args[2])
		}
		return withTracker(func(t *tracker.Tracker) error {
			data, err := t.Show(args[1], num)
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(data)
			return err
		})
	case "restore":
		if len(args) != 3 {
			return fmt.Errorf("usage: strata restore <file> <version>")
		}
		num, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid version %q", args[2])
		}
		return withTracker(func(t *tracker.Tracker) error {
			res, err := t.Restore(args[1], num)
			if err != nil {
				return err
			}
			if res.CreatedSafetyNet {
				fmt.Printf("saved current content as version %d\n", res.SafetyVersionNum)
			}
			fmt.Printf("restored version %d to %s\n", res.RestoredVersion, res.Path)
			return nil
		})
	case "export":
		if len(args) != 4 {
			return fmt.Errorf("usage: strata export <file> <version> <dest>")
		}
		num, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid version %q", args[2])
		}
		return withTracker(func(t *tracker.Tracker) error {
			if err := t.Export(args[1], num, args[3]); err != nil {
				return err
			}
			abs, _ := filepath.Abs(args[3])
			fmt.Printf("exported version %d to %s\n", num, abs)
			return nil
		})
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
	}
}

func usageText() string {
	return `usage:
  strata add <file>
  strata list
  strata remove <file>
  strata run
  strata history <file>
  strata show <file> <version>
  strata restore <file> <version>
  strata export <file> <version> <dest>
  strata watch <file>
  strata version`
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
