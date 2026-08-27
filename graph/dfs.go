/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph

import "fmt"

// DFSPaths answers path queries from a source vertex s using depth-first
// search (Sedgwick's DepthFirstPaths).  It is an immutable snapshot of the
// graph at construction time: later AddEdge calls on the graph are not
// reflected.
type DFSPaths struct {
	marked []bool
	edgeTo []int
	s      int
}

// NewDFSPaths runs depth-first search from s and records a path from s to
// every reachable vertex.  It panics on a nil or empty graph or on an
// out-of-range source — a programmer error with no sane answer.
// Complexity is O(V+E) time, O(V) space.
func NewDFSPaths(g *Graph, s int) *DFSPaths {
	if g == nil || g.n == 0 {
		panic("graph: NewDFSPaths called on a nil or empty graph")
	}
	if s < 0 || s >= g.n {
		panic(fmt.Sprintf("graph: NewDFSPaths called with out-of-range source %d (graph has %d vertices)", s, g.n))
	}
	p := &DFSPaths{marked: make([]bool, g.n), edgeTo: make([]int, g.n), s: s}
	p.dfs(g.adj, s)
	return p
}

// dfs is the recursive depth-first search over adjacency lists.
func (p *DFSPaths) dfs(adj [][]int, v int) {
	p.marked[v] = true
	for _, w := range adj[v] {
		if !p.marked[w] {
			p.edgeTo[w] = v
			p.dfs(adj, w)
		}
	}
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (p *DFSPaths) HasPathTo(v int) bool {
	if p == nil || v < 0 || v >= len(p.marked) {
		return false
	}
	return p.marked[v]
}

// PathTo returns a path from the source to v, source first, as a fresh
// slice that the caller may mutate; ok is false if there is no path (or v
// is out of range).
// Complexity is O(path length).
func (p *DFSPaths) PathTo(v int) (path []int, ok bool) {
	if !p.HasPathTo(v) {
		return nil, false
	}
	for x := v; x != p.s; x = p.edgeTo[x] {
		path = append(path, x)
	}
	path = append(path, p.s)
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, true
}
