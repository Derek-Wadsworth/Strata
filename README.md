# Strata

Personal file version tracker for local work. Strata watches files you choose, saves a new version on each change, shows diffs between versions, reports editing statistics, and links related files into a graph.

## Goals

- Track selected files and automatically create a version on each save
- Browse version history and restore older versions
- Show visual diffs between any two versions
- Provide statistics about how files and projects evolve
- Link related files (code, docs, plans) and display them as a graph

## Non-goals (for now)

- Cloud sync or multi-user collaboration
- Replacing Git — this is a personal, automatic, save-level history tool
- Merging branches (create/switch is supported; merge is not)
- Semantic diffing of binary formats (images, PDFs can be versioned and restored, but not line-diffed)

## Architecture

```
+-------------------------------------------------+
| Frontends                                       |
| CLI (cmd/strata)             GUI (Wails)        |
+------------------------+------------------------+
                         |
+------------------------v------------------------+
| Core library (internal/...)                     |
| tracker · watcher · store · db · diff · stats · |
| graph                                           |
+-------------------------------------------------+
```

The core library knows nothing about the CLI or GUI. Both frontends call the same functions, so the GUI can land later without rewriting the engine.

| Concern | Choice |
| --- | --- |
| Language | Go |
| File watching | fsnotify |
| Metadata | SQLite (modernc.org/sqlite) |
| Snapshot content | Content-addressed files (SHA-256) |
| Text diff | sergi/go-diff |
| GUI | Wails |

## Quick start

### CLI

```bash
go run ./cmd/strata version
go run ./cmd/strata add path\to\file.txt
go run ./cmd/strata list
go run ./cmd/strata run
go run ./cmd/strata start
go run ./cmd/strata status
go run ./cmd/strata stop
go run ./cmd/strata history path\to\file.txt
go run ./cmd/strata branch list path\to\file.txt
go run ./cmd/strata branch create path\to\file.txt experiment
go run ./cmd/strata branch switch path\to\file.txt main
go run ./cmd/strata title path\to\file.txt 3 "rename helper"
go run ./cmd/strata diff path\to\file.txt
go run ./cmd/strata diff path\to\file.txt 1 2 --stat
go run ./cmd/strata show path\to\file.txt 1
go run ./cmd/strata restore path\to\file.txt 1
go run ./cmd/strata export path\to\file.txt 1 path\to\copy.txt
go run ./cmd/strata remove path\to\file.txt
go run ./cmd/strata watch path\to\file.txt
```

### Desktop GUI (Wails)

```bash
# from repo root
wails dev          # development
wails build        # produces build/bin/strata-gui.exe
```

The GUI uses the same `api.Client` as the CLI with a GitHub Desktop–style layout: Files / Stats / Graph sidebar, a History column with a branch graph, side-by-side or inline diffs with folded unchanged lines, a change summary, and an Assistant panel (beta: built-in commands only, no AI model yet). Dark theme by default, light theme via the toggle in the sidebar footer. Branches and version titles are stored in SQLite (shared by CLI and GUI). While the window is open it runs its own watcher (stop the daemon first if you use `strata start`).

UI mockups: `docs/mockups/`.

Data lives under `%APPDATA%\Strata` on Windows (or `~/.strata` on macOS/Linux): `strata.db`, `objects/`, `config.json`, `strata.pid`, `strata.log`.

## Status

Branches and version titles persist in SQLite (migration 3). Create/switch branches from the GUI or CLI; new saves land on the current branch. Milestone 10 stats dashboard remains available.

## License

TBD
