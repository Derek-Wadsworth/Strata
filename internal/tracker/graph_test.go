package tracker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/store"
	"github.com/Derek-Wadsworth/Strata/internal/tracker"
)

func TestGraphLinkUnlink(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "design.md")
	bPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(aPath, []byte("# design\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	database, err := db.Open(filepath.Join(dir, "strata.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	snapshots, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	tr := tracker.New(database, snapshots, tracker.Options{})

	if _, err := tr.Add(aPath); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Add(bPath); err != nil {
		t.Fatal(err)
	}

	edge, err := tr.Link(aPath, bPath, "implements design")
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if edge.Note != "implements design" {
		t.Fatalf("note = %q", edge.Note)
	}

	g, err := tr.Graph()
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Fatalf("graph = %+v", g)
	}

	neighbors, err := tr.Neighbors(aPath)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(neighbors) != 1 || neighbors[0].Path != bPath {
		t.Fatalf("neighbors = %+v", neighbors)
	}

	if err := tr.Unlink(bPath, aPath); err != nil {
		t.Fatalf("Unlink: %v", err)
	}
	g, _ = tr.Graph()
	if len(g.Edges) != 0 {
		t.Fatalf("edges after unlink = %d", len(g.Edges))
	}
}
