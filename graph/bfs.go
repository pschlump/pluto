/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph

import (
	"fmt"

	"github.com/pschlump/pluto/queue"
)

// BFSPaths answers shortest-path queries (fewest edges) from a source
// vertex s using breadth-first search (Sedgwick's BreadthFirstPaths).  It
// is an immutable snapshot of the graph at construction time: later
// AddEdge calls on the graph are not reflected.
//
// The traversal queue is pluto's own queue.Queue — an intra-pluto
// composition, like priority_queue over heap.
type BFSPaths struct {
	marked []bool
	edgeTo []int
	dist   []int
	s      int
}

// NewBFSPaths runs breadth-first search from s and records a shortest path
// from s to every reachable vertex.  It panics on a nil or empty graph or
// on an out-of-range source — a programmer error with no sane answer.
// Complexity is O(V+E) time, O(V) space.
func NewBFSPaths(g *Graph, s int) *BFSPaths {
	if g == nil || g.n == 0 {
		panic("graph: NewBFSPaths called on a nil or empty graph")
	}
	if s < 0 || s >= g.n {
		panic(fmt.Sprintf("graph: NewBFSPaths called with out-of-range source %d (graph has %d vertices)", s, g.n))
	}
	p := &BFSPaths{marked: make([]bool, g.n), edgeTo: make([]int, g.n), dist: make([]int, g.n), s: s}
	var q queue.Queue[int]
	p.marked[s] = true
	q.Push(s)
	for !q.IsEmpty() {
		v, _ := q.Dequeue()
		for _, w := range g.adj[v] {
			if !p.marked[w] {
				p.marked[w] = true
				p.edgeTo[w] = v
				p.dist[w] = p.dist[v] + 1
				q.Push(w)
			}
		}
	}
	return p
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (p *BFSPaths) HasPathTo(v int) bool {
	if p == nil || v < 0 || v >= len(p.marked) {
		return false
	}
	return p.marked[v]
}

// PathTo returns a shortest path from the source to v, source first, as a
// fresh slice that the caller may mutate; ok is false if there is no path
// (or v is out of range).
// Complexity is O(path length).
func (p *BFSPaths) PathTo(v int) (path []int, ok bool) {
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

// DistTo returns the number of edges on a shortest path from the source to
// v; ok is false if there is no path (or v is out of range).
// Complexity is O(1).
func (p *BFSPaths) DistTo(v int) (dist int, ok bool) {
	if !p.HasPathTo(v) {
		return 0, false
	}
	return p.dist[v], true
}
