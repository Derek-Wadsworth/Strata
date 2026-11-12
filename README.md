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
- Merging versions or branching
- Semantic diffing of binary formats (images, PDFs can be versioned and restored, but not line-diffed)

## Architecture

```
+-------------------------------------------------+
| Frontends                                       |
| CLI (cmd/strata)             GUI (Wails, later) |
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
| GUI (later) | Wails |

## Status

Early setup. Building milestone by milestone, CLI first, GUI last.

## License

TBD
