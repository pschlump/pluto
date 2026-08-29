/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

import (
	"fmt"

	"github.com/pschlump/pluto/queue"
)

// BFSDirectedPaths answers shortest-path queries (fewest edges) from a
// source vertex s on a digraph using breadth-first search (Sedgwick's
// BreadthFirstDirectedPaths).  It is an immutable snapshot of the digraph
// at construction time: later AddEdge calls on the digraph are not
// reflected.
//
// The traversal queue is pluto's own queue.Queue — an intra-pluto
// composition, like priority_queue over heap.
type BFSDirectedPaths struct {
	marked []bool
	edgeTo []int
	dist   []int
	s      int
}

// NewBFSDirectedPaths runs breadth-first search from s and records a
// shortest (directed) path from s to every reachable vertex.  It panics
// on a nil or empty digraph or on an out-of-range source — a programmer
// error with no sane answer.
// Complexity is O(V+E) time, O(V) space.
func NewBFSDirectedPaths(g *Digraph, s int) *BFSDirectedPaths {
	if g == nil || g.n == 0 {
		panic("digraph: NewBFSDirectedPaths called on a nil or empty digraph")
	}
	if s < 0 || s >= g.n {
		panic(fmt.Sprintf("digraph: NewBFSDirectedPaths called with out-of-range source %d (digraph has %d vertices)", s, g.n))
	}
	p := &BFSDirectedPaths{marked: make([]bool, g.n), edgeTo: make([]int, g.n), dist: make([]int, g.n), s: s}
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

// HasPathTo reports whether there is a directed path from the source to
// v.  Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (p *BFSDirectedPaths) HasPathTo(v int) bool {
	if p == nil || v < 0 || v >= len(p.marked) {
		return false
	}
	return p.marked[v]
}

// PathTo returns a shortest directed path from the source to v, source
// first, as a fresh slice that the caller may mutate; ok is false if
// there is no path (or v is out of range).
// Complexity is O(path length).
func (p *BFSDirectedPaths) PathTo(v int) (path []int, ok bool) {
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

// DistTo returns the number of edges on a shortest directed path from the
// source to v; ok is false if there is no path (or v is out of range).
// Complexity is O(1).
func (p *BFSDirectedPaths) DistTo(v int) (dist int, ok bool) {
	if !p.HasPathTo(v) {
		return 0, false
	}
	return p.dist[v], true
}
