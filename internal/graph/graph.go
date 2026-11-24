package graph

import "time"

// Node is one tracked file in the relationship graph.
type Node struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	VersionCount int    `json:"versionCount"`
	Missing      bool   `json:"missing"`
}

// Edge is an undirected link between two tracked files.
type Edge struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

// Graph is the full set of active files and their links.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
