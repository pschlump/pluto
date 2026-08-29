/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

import "fmt"

// DirectedDFS answers multi-source reachability queries on a digraph
// (Sedgwick's DirectedDFS, §4.4): the set of vertices reachable from any
// of the sources.  It is an immutable snapshot of the digraph at
// construction time: later AddEdge calls on the digraph are not
// reflected, and a DirectedDFS is safe for concurrent reads.
type DirectedDFS struct {
	marked []bool
	count  int
}

// NewDirectedDFS snapshots the digraph's adjacency lists under the read
// lock, then runs depth-first search from every source lock-free on the
// snapshot, recording the reachable vertices.  It panics on a nil or
// empty digraph, with no sources, or on an out-of-range source — a
// programmer error with no sane answer.
// Complexity is O(V+E) time, O(V) space.
func NewDirectedDFS(g *Digraph, sources ...int) *DirectedDFS {
	if g == nil {
		panic("digraph_ts: NewDirectedDFS called on a nil digraph")
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		panic("digraph_ts: NewDirectedDFS called on an empty digraph")
	}
	if len(sources) == 0 {
		g.lock.RUnlock()
		panic("digraph_ts: NewDirectedDFS called with no sources, need at least one")
	}
	for _, s := range sources {
		if s < 0 || s >= g.n {
			g.lock.RUnlock()
			panic(fmt.Sprintf("digraph_ts: NewDirectedDFS called with out-of-range source %d (digraph has %d vertices)", s, g.n))
		}
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	d := &DirectedDFS{marked: make([]bool, len(adj))}
	for _, s := range sources {
		if !d.marked[s] {
			d.dfs(adj, s)
		}
	}
	return d
}

// dfs is the recursive depth-first search over the snapshot adjacency
// lists; no lock is held while it runs.
func (d *DirectedDFS) dfs(adj [][]int, v int) {
	d.marked[v] = true
	d.count++
	for _, w := range adj[v] {
		if !d.marked[w] {
			d.dfs(adj, w)
		}
	}
}

// Marked reports whether v is reachable from any of the sources.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (d *DirectedDFS) Marked(v int) bool {
	if d == nil || v < 0 || v >= len(d.marked) {
		return false
	}
	return d.marked[v]
}

// Count returns the number of vertices reachable from any of the sources.
// Complexity is O(1).
func (d *DirectedDFS) Count() int {
	if d == nil {
		return 0
	}
	return d.count
}

// DFSDirectedPaths answers path queries from a source vertex s on a
// digraph using depth-first search (Sedgwick's DepthFirstDirectedPaths).
// It is an immutable snapshot of the digraph at construction time: later
// AddEdge calls on the digraph are not reflected, and a DFSDirectedPaths
// is safe for concurrent reads.
type DFSDirectedPaths struct {
	marked []bool
	edgeTo []int
	s      int
}

// NewDFSDirectedPaths snapshots the digraph's adjacency lists under the
// read lock, then runs depth-first search from s lock-free on the
// snapshot, recording a (directed) path from s to every reachable vertex.
// It panics on a nil or empty digraph or on an out-of-range source — a
// programmer error with no sane answer.
// Complexity is O(V+E) time, O(V) space.
func NewDFSDirectedPaths(g *Digraph, s int) *DFSDirectedPaths {
	if g == nil {
		panic("digraph_ts: NewDFSDirectedPaths called on a nil digraph")
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		panic("digraph_ts: NewDFSDirectedPaths called on an empty digraph")
	}
	if s < 0 || s >= g.n {
		g.lock.RUnlock()
		panic(fmt.Sprintf("digraph_ts: NewDFSDirectedPaths called with out-of-range source %d (digraph has %d vertices)", s, g.n))
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	p := &DFSDirectedPaths{marked: make([]bool, len(adj)), edgeTo: make([]int, len(adj)), s: s}
	p.dfs(adj, s)
	return p
}

// dfs is the recursive depth-first search over the snapshot adjacency
// lists; no lock is held while it runs.
func (p *DFSDirectedPaths) dfs(adj [][]int, v int) {
	p.marked[v] = true
	for _, w := range adj[v] {
		if !p.marked[w] {
			p.edgeTo[w] = v
			p.dfs(adj, w)
		}
	}
}

// HasPathTo reports whether there is a directed path from the source to
// v.  Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (p *DFSDirectedPaths) HasPathTo(v int) bool {
	if p == nil || v < 0 || v >= len(p.marked) {
		return false
	}
	return p.marked[v]
}

// PathTo returns a directed path from the source to v, source first, as a
// fresh slice that the caller may mutate; ok is false if there is no path
// (or v is out of range).
// Complexity is O(path length).
func (p *DFSDirectedPaths) PathTo(v int) (path []int, ok bool) {
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
