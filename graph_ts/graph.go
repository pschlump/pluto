/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package graph_ts implements an undirected graph on the integer vertices
// 0..n-1 with adjacency lists, safe for concurrent use, plus the three
// fundamental graph queries — depth-first paths, breadth-first paths, and
// connected components — as constructor-style query objects (Sedgwick,
// Algorithms, 4th ed., §4.1).  It is the thread-safe twin of
// github.com/pschlump/pluto/graph — the same API, guarded by a
// sync.RWMutex.
//
// Concurrency model:
//
//	AddEdge takes the write lock.
//	V, Len, E, Degree, HasEdge take the read lock and release it before
//	returning, so they run in parallel with each other.
//	Adj operates on a snapshot of the one adjacency list copied under
//	the read lock when Adj is called, so it is safe to use concurrently
//	with any graph operation — including mutating the graph from inside
//	the loop — and never observes later modifications.
//	NewDFSPaths, NewBFSPaths, and NewCC snapshot the whole adjacency
//	structure under the read lock, then run the traversal lock-free on
//	the snapshot — no lock is held during the traversal.
//
// The query objects (DFSPaths, BFSPaths, CC) are immutable snapshots of
// the graph at construction time: later AddEdge calls are not reflected,
// and the query objects themselves are safe for concurrent reads.
//
// Self-loops and parallel edges are allowed: AddEdge(v, v) counts once in
// E and twice in Degree(v), exactly like Sedgwick.  There is deliberately
// no RemoveEdge — matching Sedgwick's Graph and keeping the surface
// minimal.
//
// Adj yields neighbors in insertion order (deterministic — there is no
// random seed).
//
// Panic contract (each panic message names the method and the fix):
// NewGraph with n < 1; AddEdge on a nil *Graph; NewDFSPaths/NewBFSPaths on
// a nil or empty graph or with an out-of-range source.  Every other
// operation tolerates a nil or zero-value graph as an empty graph with no
// vertices (a zero-value graph reports all vertices out of range, so
// AddEdge on it returns false).
//
// Run the tests with -race.
package graph_ts

import (
	"fmt"
	"iter"
	"sync"
)

// Graph is an undirected graph on the vertices 0..n-1, stored as adjacency
// lists, safe for concurrent use.
type Graph struct {
	n    int
	e    int
	adj  [][]int
	lock sync.RWMutex
}

// NewGraph creates a graph with n vertices (0..n-1) and no edges.
// It panics if n < 1.
// Complexity is O(n).
func NewGraph(n int) *Graph {
	if n < 1 {
		panic(fmt.Sprintf("graph_ts: NewGraph called with n=%d, need n >= 1", n))
	}
	return &Graph{n: n, adj: make([][]int, n)}
}

// AddEdge adds the edge v-w to the graph and returns true.  It returns
// false if either vertex is out of range.  Self-loops and parallel edges
// are allowed; a self-loop counts once in E and twice in Degree(v).
// It panics on a nil graph — a nil graph cannot store an edge.
// Complexity is O(1).
func (g *Graph) AddEdge(v, w int) bool {
	if g == nil {
		panic("graph_ts: AddEdge called on a nil graph")
	}
	g.lock.Lock()
	defer g.lock.Unlock()
	if v < 0 || v >= g.n || w < 0 || w >= g.n {
		return false
	}
	g.adj[v] = append(g.adj[v], w)
	g.adj[w] = append(g.adj[w], v)
	g.e++
	return true
}

// V returns the number of vertices in the graph.
// Complexity is O(1).
func (g *Graph) V() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.n
}

// Len is an alias for V, matching the other pluto packages.
// Complexity is O(1).
func (g *Graph) Len() int {
	return g.V()
}

// E returns the number of edges in the graph (a self-loop counts once).
// Complexity is O(1).
func (g *Graph) E() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.e
}

// Degree returns the number of edges incident to v; ok is false if v is
// out of range (a self-loop counts twice).
// Complexity is O(1).
func (g *Graph) Degree(v int) (degree int, ok bool) {
	if g == nil {
		return 0, false
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	if v < 0 || v >= g.n {
		return 0, false
	}
	return len(g.adj[v]), true
}

// HasEdge reports whether the edge v-w exists.  Out-of-range vertices
// report false.
// Complexity is O(degree(v)).
func (g *Graph) HasEdge(v, w int) bool {
	if g == nil {
		return false
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	if v < 0 || v >= g.n || w < 0 || w >= g.n {
		return false
	}
	for _, x := range g.adj[v] {
		if x == w {
			return true
		}
	}
	return false
}

// Adj returns a range-over-func iterator that yields the neighbors of v in
// insertion order.
//
//	for w := range g.Adj(v) {
//		...
//	}
//
// An out-of-range v (or a nil graph) yields nothing.  The iterator
// operates on a snapshot of the adjacency list copied under the read lock
// when Adj is called, so it is safe to call other graph operations —
// including from inside the loop — and it never observes later
// modifications.
// Complexity is O(degree(v)).
func (g *Graph) Adj(v int) iter.Seq[int] {
	if g == nil {
		return func(func(int) bool) {} // a nil graph iterates as an empty one
	}
	g.lock.RLock()
	if v < 0 || v >= g.n {
		g.lock.RUnlock()
		return func(func(int) bool) {} // out-of-range iterates as empty
	}
	snapshot := append([]int(nil), g.adj[v]...)
	g.lock.RUnlock()
	return func(yield func(int) bool) {
		for _, w := range snapshot {
			if !yield(w) {
				return
			}
		}
	}
}

// snapshotAdj returns a deep copy of the adjacency lists; the caller must
// hold the read lock.
func (g *Graph) snapshotAdj() [][]int {
	adj := make([][]int, g.n)
	for i := range g.adj {
		adj[i] = append([]int(nil), g.adj[i]...)
	}
	return adj
}
