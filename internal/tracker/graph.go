package tracker

import (
	"fmt"
	"path/filepath"

	"github.com/Derek-Wadsworth/Strata/internal/db"
	"github.com/Derek-Wadsworth/Strata/internal/graph"
)

// Graph returns active tracked files and the links between them.
func (t *Tracker) Graph() (graph.Graph, error) {
	files, err := t.List()
	if err != nil {
		return graph.Graph{}, err
	}
	links, err := t.db.ListActiveLinks()
	if err != nil {
		return graph.Graph{}, err
	}

	nodes := make([]graph.Node, 0, len(files))
	for _, f := range files {
		nodes = append(nodes, graph.Node{
			Path:         f.Path,
			Name:         filepath.Base(f.Path),
			VersionCount: f.VersionCount,
			Missing:      f.Missing,
		})
	}
	edges := make([]graph.Edge, 0, len(links))
	for _, l := range links {
		edges = append(edges, graph.Edge{
			From:      l.APath,
			To:        l.BPath,
			Note:      l.Note,
			CreatedAt: l.CreatedAt,
		})
	}
	return graph.Graph{Nodes: nodes, Edges: edges}, nil
}

// Link relates two tracked files. note may be empty.
func (t *Tracker) Link(pathA, pathB, note string) (graph.Edge, error) {
	a, err := t.activeFileByPath(pathA)
	if err != nil {
		return graph.Edge{}, err
	}
	b, err := t.activeFileByPath(pathB)
	if err != nil {
		return graph.Edge{}, err
	}
	link, err := t.db.LinkFiles(a.ID, b.ID, note)
	if err != nil {
		return graph.Edge{}, err
	}
	return graph.Edge{
		From:      link.APath,
		To:        link.BPath,
		Note:      link.Note,
		CreatedAt: link.CreatedAt,
	}, nil
}

// Unlink removes the relationship between two tracked files.
func (t *Tracker) Unlink(pathA, pathB string) error {
	a, err := t.activeFileByPath(pathA)
	if err != nil {
		return err
	}
	b, err := t.activeFileByPath(pathB)
	if err != nil {
		return err
	}
	return t.db.UnlinkFiles(a.ID, b.ID)
}

// Neighbors returns active files linked to path.
func (t *Tracker) Neighbors(path string) ([]graph.Node, error) {
	f, err := t.activeFileByPath(path)
	if err != nil {
		return nil, err
	}
	links, err := t.db.ListLinksForFile(f.ID)
	if err != nil {
		return nil, err
	}
	active, err := t.List()
	if err != nil {
		return nil, err
	}
	byPath := map[string]ListedFile{}
	for _, item := range active {
		byPath[item.Path] = item
	}
	out := make([]graph.Node, 0, len(links))
	seen := map[string]bool{}
	for _, l := range links {
		other := l.APath
		if l.AID == f.ID {
			other = l.BPath
		}
		if seen[other] {
			continue
		}
		item, ok := byPath[other]
		if !ok {
			continue
		}
		seen[other] = true
		out = append(out, graph.Node{
			Path:         item.Path,
			Name:         filepath.Base(item.Path),
			VersionCount: item.VersionCount,
			Missing:      item.Missing,
		})
	}
	return out, nil
}

func (t *Tracker) activeFileByPath(path string) (db.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return db.File{}, fmt.Errorf("resolve path: %w", err)
	}
	return t.db.GetActiveFileByPath(abs)
}
