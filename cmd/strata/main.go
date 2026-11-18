package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/appdata"
	"github.com/Derek-Wadsworth/Strata/internal/config"
	"github.com/Derek-Wadsworth/Strata/internal/daemon"
	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/diff"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
	"github.com/Derek-Wadsworth/Strata/internal/watcher"
)

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
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
			fmt.Fprintln(w, "VERSIONS\tSTATUS\tPATH")
			for _, f := range files {
				status := "ok"
				if f.Missing {
					status = "missing"
				}
				fmt.Fprintf(w, "%d\t%s\t%s\n", f.VersionCount, status, f.Path)
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
			return runForeground(t, true)
		})
	case "start":
		dataDir, err := resolveDataDir()
		if err != nil {
			return err
		}
		st, err := daemon.Start(dataDir)
		if err != nil {
			return err
		}
		fmt.Printf("daemon started (pid %d)\n", st.PID)
		fmt.Printf("log: %s\n", appdata.LogPath(dataDir))
		return nil
	case "stop":
		dataDir, err := resolveDataDir()
		if err != nil {
			return err
		}
		if err := daemon.Stop(dataDir); err != nil {
			return err
		}
		fmt.Println("daemon stopped")
		return nil
	case "status":
		dataDir, err := resolveDataDir()
		if err != nil {
			return err
		}
		st, err := daemon.ReadStatus(dataDir)
		if err != nil {
			return err
		}
		if st.Running {
			fmt.Printf("running (pid %d)\n", st.PID)
		} else {
			fmt.Println("not running")
		}
		return nil
	case "daemon":
		// Internal entrypoint used by `strata start`.
		return runDaemon()
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
			fmt.Fprintln(w, "VERSION\tTIME\tSIZE\tCHANGES")
			for _, e := range entries {
				fmt.Fprintf(w, "%d\t%s\t%d\t%s\n",
					e.Number,
					e.CreatedAt.Local().Format(time.RFC3339),
					e.Size,
					formatLineStats(e.LinesAdded, e.LinesRemoved),
				)
			}
			return w.Flush()
		})
	case "diff":
		path, fromVer, toVer, statOnly, err := parseDiffArgs(args[1:])
		if err != nil {
			return err
		}
		return withTracker(func(t *tracker.Tracker) error {
			res, err := t.Diff(path, fromVer, toVer)
			if err != nil {
				return err
			}
			return printDiff(res, statOnly)
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
  strata start
  strata stop
  strata status
  strata history <file>
  strata diff <file> [--stat]
  strata diff <file> <v1> <v2> [--stat]
  strata show <file> <version>
  strata restore <file> <version>
  strata export <file> <version> <dest>
  strata watch <file>
  strata version`
}

func parseDiffArgs(args []string) (path string, fromVer, toVer int, statOnly bool, err error) {
	var positional []string
	for _, a := range args {
		if a == "--stat" {
			statOnly = true
			continue
		}
		positional = append(positional, a)
	}
	switch len(positional) {
	case 1:
		return positional[0], 0, 0, statOnly, nil
	case 3:
		fromVer, err = strconv.Atoi(positional[1])
		if err != nil {
			return "", 0, 0, false, fmt.Errorf("invalid version %q", positional[1])
		}
		toVer, err = strconv.Atoi(positional[2])
		if err != nil {
			return "", 0, 0, false, fmt.Errorf("invalid version %q", positional[2])
		}
		return positional[0], fromVer, toVer, statOnly, nil
	default:
		return "", 0, 0, false, fmt.Errorf("usage: strata diff <file> [<v1> <v2>] [--stat]")
	}
}

func printDiff(res tracker.DiffResult, statOnly bool) error {
	fmt.Printf("--- %s (v%d)\n+++ %s (v%d)\n", res.Path, res.FromVer, res.Path, res.ToVer)
	if res.Diff.Binary {
		fmt.Println("binary files differ")
		return nil
	}
	fmt.Printf("%s+%d -%d%s\n", ansiYellow, res.Diff.LinesAdded, res.Diff.LinesRemoved, ansiReset)
	if statOnly {
		return nil
	}
	for _, hunk := range res.Diff.Hunks {
		for _, line := range hunk.Lines {
			switch line.Op {
			case diff.OpAdd:
				fmt.Printf("%s+%s%s\n", ansiGreen, line.Text, ansiReset)
			case diff.OpRemove:
				fmt.Printf("%s-%s%s\n", ansiRed, line.Text, ansiReset)
			case diff.OpEqual:
				fmt.Printf(" %s\n", line.Text)
			}
		}
	}
	return nil
}

func formatLineStats(added, removed *int) string {
	if added == nil || removed == nil {
		return "-"
	}
	return fmt.Sprintf("+%d -%d", *added, *removed)
}

func withTracker(fn func(*tracker.Tracker) error) error {
	_, t, database, _, err := openTracker(nil)
	if err != nil {
		return err
	}
	defer database.Close()
	return fn(t)
}

func resolveDataDir() (string, error) {
	base, err := appdata.EnsureDir()
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(appdata.ConfigPath(base))
	if err != nil {
		return "", err
	}
	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return "", fmt.Errorf("create data dir: %w", err)
		}
		return cfg.DataDir, nil
	}
	return base, nil
}

func openTracker(log *slog.Logger) (dataDir string, t *tracker.Tracker, database *db.DB, cfg config.Config, err error) {
	base, err := appdata.EnsureDir()
	if err != nil {
		return "", nil, nil, cfg, err
	}
	cfg, err = config.Load(appdata.ConfigPath(base))
	if err != nil {
		return "", nil, nil, cfg, err
	}
	dataDir = base
	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return "", nil, nil, cfg, fmt.Errorf("create data dir: %w", err)
		}
		dataDir = cfg.DataDir
	}
	database, err = db.Open(appdata.DBPath(dataDir))
	if err != nil {
		return "", nil, nil, cfg, err
	}
	snapshots, err := store.New(dataDir)
	if err != nil {
		database.Close()
		return "", nil, nil, cfg, err
	}
	if log == nil {
		log = slog.Default()
	}
	t = tracker.New(database, snapshots, tracker.Options{
		Debounce:     cfg.Debounce(),
		MaxFileBytes: cfg.MaxFileBytes,
		Log:          log,
	})
	return dataDir, t, database, cfg, nil
}

func runForeground(t *tracker.Tracker, printCatchUp bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	results, err := t.CatchUp()
	if err != nil {
		return err
	}
	if printCatchUp {
		for _, res := range results {
			if !res.Skipped {
				fmt.Printf("catch-up version %d: %s\n", res.VersionNum, res.Path)
			}
		}
	}

	files, err := t.List()
	if err != nil {
		return err
	}
	fmt.Printf("watching %d file(s) (Ctrl+C to stop)\n", len(files))
	for _, f := range files {
		mark := ""
		if f.Missing {
			mark = " [missing]"
		}
		fmt.Printf("  %s%s\n", f.Path, mark)
	}
	return t.Run(ctx, func(res tracker.SnapshotResult) {
		fmt.Printf("version %d: %s\n", res.VersionNum, res.Path)
	})
}

func runDaemon() error {
	logFile, err := openDaemonLog()
	if err != nil {
		return err
	}
	defer logFile.Close()

	log := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	dataDir, t, database, _, err := openTracker(log)
	if err != nil {
		return err
	}
	defer database.Close()
	defer os.Remove(daemon.PidPath(dataDir))

	if err := daemon.WritePIDFile(dataDir, os.Getpid()); err != nil {
		return err
	}

	log.Info("daemon starting", "pid", os.Getpid(), "data_dir", dataDir)
	results, err := t.CatchUp()
	if err != nil {
		return err
	}
	for _, res := range results {
		if !res.Skipped {
			log.Info("catch-up version", "version", res.VersionNum, "path", res.Path)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = t.Run(ctx, func(res tracker.SnapshotResult) {
		log.Info("version recorded", "version", res.VersionNum, "path", res.Path)
	})
	log.Info("daemon stopped")
	return err
}

func openDaemonLog() (*os.File, error) {
	dataDir, err := resolveDataDir()
	if err != nil {
		return nil, err
	}
	return os.OpenFile(appdata.LogPath(dataDir), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func watchFile(path string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("watching %s (Ctrl+C to stop)\n", path)
	return watcher.Watch(ctx, path, watcher.DefaultDebounce, func(changed string) {
		fmt.Printf("saved: %s\n", changed)
	})
}
