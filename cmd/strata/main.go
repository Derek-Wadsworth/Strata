package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	default:
		return fmt.Errorf("unknown command %q\n\nusage:\n  strata watch <file>\n  strata version", args[0])
	}
}

func watchFile(path string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("watching %s (Ctrl+C to stop)\n", path)
	return watcher.Watch(ctx, path, watcher.DefaultDebounce, func(changed string) {
		fmt.Printf("saved: %s\n", changed)
	})
}
