package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/Derek-Wadsworth/Strata/internal/api"
	"github.com/Derek-Wadsworth/Strata/internal/diff"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
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
		return withClient(nil, func(c *api.Client) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			fmt.Printf("watching %s (Ctrl+C to stop)\n", args[1])
			return c.WatchFile(ctx, args[1], func(changed string) {
				fmt.Printf("saved: %s\n", changed)
			})
		})
	case "add":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata add <file>")
		}
		return withClient(nil, func(c *api.Client) error {
			n, err := c.Add(args[1])
			if err != nil {
				return err
			}
			abs, _ := c.AbsPath(args[1])
			fmt.Printf("tracking %s (version %d)\n", abs, n)
			return nil
		})
	case "list":
		return withClient(nil, func(c *api.Client) error {
			files, err := c.List()
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
		return withClient(nil, func(c *api.Client) error {
			if err := c.Remove(args[1]); err != nil {
				return err
			}
			abs, _ := c.AbsPath(args[1])
			fmt.Printf("stopped tracking %s (history kept)\n", abs)
			return nil
		})
	case "run":
		return withClient(nil, func(c *api.Client) error {
			return runForeground(c, true)
		})
	case "start":
		return withClient(nil, func(c *api.Client) error {
			st, err := c.StartDaemon()
			if err != nil {
				return err
			}
			fmt.Printf("daemon started (pid %d)\n", st.PID)
			fmt.Printf("log: %s\n", c.LogPath())
			return nil
		})
	case "stop":
		return withClient(nil, func(c *api.Client) error {
			if err := c.StopDaemon(); err != nil {
				return err
			}
			fmt.Println("daemon stopped")
			return nil
		})
	case "status":
		return withClient(nil, func(c *api.Client) error {
			st, err := c.DaemonStatus()
			if err != nil {
				return err
			}
			if st.Running {
				fmt.Printf("running (pid %d)\n", st.PID)
			} else {
				fmt.Println("not running")
			}
			return nil
		})
	case "daemon":
		return runDaemon()
	case "history":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata history <file>")
		}
		return withClient(nil, func(c *api.Client) error {
			entries, err := c.History(args[1])
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("no versions")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "VERSION\tBRANCH\tTIME\tSIZE\tCHANGES\tTITLE")
			for _, e := range entries {
				title := e.Title
				if title == "" {
					title = "-"
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\n",
					e.Number,
					e.Branch,
					e.CreatedAt.Local().Format(time.RFC3339),
					e.Size,
					formatLineStats(e.LinesAdded, e.LinesRemoved),
					title,
				)
			}
			return w.Flush()
		})
	case "branch":
		return runBranch(args[1:])
	case "link":
		if len(args) < 3 {
			return fmt.Errorf("usage: strata link <file-a> <file-b> [note]")
		}
		note := ""
		if len(args) > 3 {
			note = args[3]
			for _, part := range args[4:] {
				note += " " + part
			}
		}
		return withClient(nil, func(c *api.Client) error {
			edge, err := c.Link(args[1], args[2], note)
			if err != nil {
				return err
			}
			fmt.Printf("linked %s ↔ %s", edge.From, edge.To)
			if edge.Note != "" {
				fmt.Printf(" (%s)", edge.Note)
			}
			fmt.Println()
			return nil
		})
	case "unlink":
		if len(args) != 3 {
			return fmt.Errorf("usage: strata unlink <file-a> <file-b>")
		}
		return withClient(nil, func(c *api.Client) error {
			if err := c.Unlink(args[1], args[2]); err != nil {
				return err
			}
			a, _ := c.AbsPath(args[1])
			b, _ := c.AbsPath(args[2])
			fmt.Printf("unlinked %s ↔ %s\n", a, b)
			return nil
		})
	case "graph":
		return withClient(nil, func(c *api.Client) error {
			g, err := c.Graph()
			if err != nil {
				return err
			}
			if len(g.Nodes) == 0 {
				fmt.Println("no tracked files")
				return nil
			}
			fmt.Printf("%d file(s), %d link(s)\n", len(g.Nodes), len(g.Edges))
			for _, n := range g.Nodes {
				mark := ""
				if n.Missing {
					mark = " [missing]"
				}
				fmt.Printf("  %s (%d versions)%s\n", n.Path, n.VersionCount, mark)
			}
			if len(g.Edges) == 0 {
				fmt.Println("no links")
				return nil
			}
			fmt.Println("links:")
			for _, e := range g.Edges {
				if e.Note != "" {
					fmt.Printf("  %s ↔ %s — %s\n", e.From, e.To, e.Note)
				} else {
					fmt.Printf("  %s ↔ %s\n", e.From, e.To)
				}
			}
			return nil
		})
	case "title":
		if len(args) < 4 {
			return fmt.Errorf("usage: strata title <file> <version> <title>")
		}
		num, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid version %q", args[2])
		}
		title := args[3]
		for _, part := range args[4:] {
			title += " " + part
		}
		return withClient(nil, func(c *api.Client) error {
			if err := c.SetVersionTitle(args[1], num, title); err != nil {
				return err
			}
			abs, _ := c.AbsPath(args[1])
			fmt.Printf("set title on %s v%d\n", abs, num)
			return nil
		})
	case "diff":
		path, fromVer, toVer, statOnly, err := parseDiffArgs(args[1:])
		if err != nil {
			return err
		}
		return withClient(nil, func(c *api.Client) error {
			res, err := c.Diff(path, fromVer, toVer)
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
		return withClient(nil, func(c *api.Client) error {
			data, err := c.Show(args[1], num)
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
		return withClient(nil, func(c *api.Client) error {
			res, err := c.Restore(args[1], num)
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
		return withClient(nil, func(c *api.Client) error {
			if err := c.Export(args[1], num, args[3]); err != nil {
				return err
			}
			abs, _ := c.AbsPath(args[3])
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

func runBranch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: strata branch list <file>\n       strata branch create <file> <name> [from]\n       strata branch switch <file> <name>")
	}
	switch args[0] {
	case "list":
		if len(args) != 2 {
			return fmt.Errorf("usage: strata branch list <file>")
		}
		return withClient(nil, func(c *api.Client) error {
			branches, err := c.ListBranches(args[1])
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tFROM\tSTATUS")
			for _, b := range branches {
				status := ""
				if b.Current {
					status = "current"
				}
				fmt.Fprintf(w, "%s\tv%d\t%s\n", b.Name, b.FromVersion, status)
			}
			return w.Flush()
		})
	case "create":
		if len(args) < 3 || len(args) > 4 {
			return fmt.Errorf("usage: strata branch create <file> <name> [from]")
		}
		fromVer := 0
		if len(args) == 4 {
			n, err := strconv.Atoi(args[3])
			if err != nil {
				return fmt.Errorf("invalid from version %q", args[3])
			}
			fromVer = n
		}
		return withClient(nil, func(c *api.Client) error {
			if fromVer == 0 {
				entries, err := c.History(args[1])
				if err != nil {
					return err
				}
				if len(entries) > 0 {
					fromVer = entries[len(entries)-1].Number
				}
			}
			b, err := c.CreateBranch(args[1], args[2], fromVer)
			if err != nil {
				return err
			}
			abs, _ := c.AbsPath(args[1])
			fmt.Printf("created branch %s on %s (from v%d, now current)\n", b.Name, abs, b.FromVersion)
			return nil
		})
	case "switch", "checkout":
		if len(args) != 3 {
			return fmt.Errorf("usage: strata branch switch <file> <name>")
		}
		return withClient(nil, func(c *api.Client) error {
			if err := c.CheckoutBranch(args[1], args[2]); err != nil {
				return err
			}
			abs, _ := c.AbsPath(args[1])
			fmt.Printf("switched %s to %s\n", abs, args[2])
			return nil
		})
	default:
		return fmt.Errorf("unknown branch command %q\nusage: strata branch list|create|switch ...", args[0])
	}
}

func withClient(log *slog.Logger, fn func(*api.Client) error) error {
	c, err := api.Open(log)
	if err != nil {
		return err
	}
	defer c.Close()
	return fn(c)
}

func runForeground(c *api.Client, printCatchUp bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	results, err := c.CatchUp()
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

	files, err := c.List()
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
	return c.Run(ctx, func(res tracker.SnapshotResult) {
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

	c, err := api.Open(log)
	if err != nil {
		return err
	}
	defer c.Close()
	defer os.Remove(c.PidPath())

	if err := c.WriteDaemonPID(os.Getpid()); err != nil {
		return err
	}

	log.Info("daemon starting", "pid", os.Getpid(), "data_dir", c.DataDir())
	results, err := c.CatchUp()
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
	err = c.Run(ctx, func(res tracker.SnapshotResult) {
		log.Info("version recorded", "version", res.VersionNum, "path", res.Path)
	})
	log.Info("daemon stopped")
	return err
}

func openDaemonLog() (*os.File, error) {
	c, err := api.Open(nil)
	if err != nil {
		return nil, err
	}
	path := c.LogPath()
	_ = c.Close()
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}
