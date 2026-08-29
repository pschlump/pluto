/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package digraph_ts implements a directed graph on the integer vertices
// 0..n-1 with adjacency lists, safe for concurrent use, plus the
// fundamental digraph algorithms — depth-first reachability and paths,
// breadth-first paths, cycle detection, depth-first order, topological
// sort, and Kosaraju's strong components — as constructor-style query
// objects (Sedgwick, Algorithms, 4th ed., §4.4–4.6).  It is the
// thread-safe twin of github.com/pschlump/pluto/digraph — the same API,
// guarded by a sync.RWMutex.
//
// Concurrency model:
//
//	AddEdge takes the write lock.
//	V, Len, E, OutDegree, InDegree, HasEdge take the read lock and
//	release it before returning, so they run in parallel with each
//	other.
//	Adj operates on a snapshot of the one adjacency list copied under
//	the read lock when Adj is called, so it is safe to use concurrently
//	with any digraph operation — including mutating the digraph from
//	inside the loop — and never observes later modifications.
//	Reverse snapshots the adjacency under the read lock and builds the
//	reversed digraph as an independent copy.
//	NewDirectedDFS, NewDFSDirectedPaths, NewBFSDirectedPaths,
//	NewDirectedCycle, NewDepthFirstOrder, NewTopological, and
//	NewKosarajuSCC snapshot the whole adjacency structure under the
//	read lock, then run the traversal lock-free on the snapshot — no
//	lock is held during the traversal.
//
// The query objects (DirectedDFS, DFSDirectedPaths, BFSDirectedPaths,
// DirectedCycle, DepthFirstOrder, Topological, KosarajuSCC) are immutable
// snapshots of the digraph at construction time: later AddEdge calls are
// not reflected, and the query objects themselves are safe for concurrent
// reads.
//
// Self-loops and parallel edges are allowed: AddEdge(v, v) counts once in
// E, once in OutDegree(v), and once in InDegree(v).  There is
// deliberately no RemoveEdge — matching Sedgwick's Digraph and keeping
// the surface minimal.
//
// Adj yields out-neighbors in insertion order (deterministic — there is
// no random seed).
//
// Panic contract (each panic message names the method and the fix):
// NewDigraph with n < 1; AddEdge on a nil *Digraph;
// NewDirectedDFS/NewDFSDirectedPaths/NewBFSDirectedPaths on a nil or
// empty graph or with an out-of-range source (NewDirectedDFS also panics
// with no sources).  NewDirectedCycle/NewDepthFirstOrder/NewTopological/
// NewKosarajuSCC have a sane answer for a nil or empty graph — no cycle,
// an empty order, no components — so they do not panic.  Every other
// operation tolerates a nil or zero-value digraph as an empty digraph
// with no vertices (a zero-value digraph reports all vertices out of
// range, so AddEdge on it returns false).
//
// Run the tests with -race.
package digraph_ts

import (
	"fmt"
	"iter"
	"sync"
)

// Digraph is a directed graph on the vertices 0..n-1, stored as adjacency
// lists, safe for concurrent use.  The in-degree of every vertex is
// maintained alongside the adjacency lists, so InDegree is O(1).
type Digraph struct {
	n     int
	e     int
	adj   [][]int
	indeg []int
	lock  sync.RWMutex
}

// NewDigraph creates a digraph with n vertices (0..n-1) and no edges.
// It panics if n < 1.
// Complexity is O(n).
func NewDigraph(n int) *Digraph {
	if n < 1 {
		panic(fmt.Sprintf("digraph_ts: NewDigraph called with n=%d, need n >= 1", n))
	}
	return &Digraph{n: n, adj: make([][]int, n), indeg: make([]int, n)}
}

// AddEdge adds the directed edge v->w to the digraph and returns true.
// It returns false if either vertex is out of range.  Self-loops and
// parallel edges are allowed.
// It panics on a nil digraph — a nil digraph cannot store an edge.
// Complexity is O(1).
func (g *Digraph) AddEdge(v, w int) bool {
	if g == nil {
		panic("digraph_ts: AddEdge called on a nil digraph")
	}
	g.lock.Lock()
	defer g.lock.Unlock()
	if v < 0 || v >= g.n || w < 0 || w >= g.n {
		return false
	}
	g.adj[v] = append(g.adj[v], w)
	g.indeg[w]++
	g.e++
	return true
}

// V returns the number of vertices in the digraph.
// Complexity is O(1).
func (g *Digraph) V() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.n
}

// Len is an alias for V, matching the other pluto packages.
// Complexity is O(1).
func (g *Digraph) Len() int {
	return g.V()
}

// E returns the number of edges in the digraph (a self-loop counts once).
// Complexity is O(1).
func (g *Digraph) E() int {
	if g == nil {
		return 0
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	return g.e
}

// OutDegree returns the number of edges leaving v; ok is false if v is
// out of range.
// Complexity is O(1).
func (g *Digraph) OutDegree(v int) (degree int, ok bool) {
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

// InDegree returns the number of edges entering v; ok is false if v is
// out of range.
// Complexity is O(1).
func (g *Digraph) InDegree(v int) (degree int, ok bool) {
	if g == nil {
		return 0, false
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	if v < 0 || v >= g.n {
		return 0, false
	}
	return g.indeg[v], true
}

// HasEdge reports whether the directed edge v->w exists.  Out-of-range
// vertices report false.
// Complexity is O(outdegree(v)).
func (g *Digraph) HasEdge(v, w int) bool {
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

// Adj returns a range-over-func iterator that yields the out-neighbors of
// v in insertion order.
//
//	for w := range g.Adj(v) {
//		...
//	}
//
// An out-of-range v (or a nil digraph) yields nothing.  The iterator
// operates on a snapshot of the adjacency list copied under the read lock
// when Adj is called, so it is safe to call other digraph operations —
// including from inside the loop — and it never observes later
// modifications.
// Complexity is O(outdegree(v)).
func (g *Digraph) Adj(v int) iter.Seq[int] {
	if g == nil {
		return func(func(int) bool) {} // a nil digraph iterates as an empty one
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

// Reverse returns a new digraph with the same vertices and every edge
// flipped (v->w becomes w->v), built from a snapshot of the adjacency
// taken under the read lock.  A nil digraph reverses to nil.  The
// original digraph is not modified; the reverse is an independent copy.
// Complexity is O(V+E).
func (g *Digraph) Reverse() *Digraph {
	if g == nil {
		return nil
	}
	g.lock.RLock()
	defer g.lock.RUnlock()
	r := &Digraph{n: g.n, e: g.e, adj: make([][]int, g.n), indeg: make([]int, g.n)}
	for v := 0; v < g.n; v++ {
		r.indeg[v] = len(g.adj[v]) // in-degree of the reverse = out-degree here
		for _, w := range g.adj[v] {
			r.adj[w] = append(r.adj[w], v)
		}
	}
	return r
}

// snapshotAdj returns a deep copy of the adjacency lists; the caller must
// hold the read lock.
func (g *Digraph) snapshotAdj() [][]int {
	adj := make([][]int, g.n)
	for i := range g.adj {
		adj[i] = append([]int(nil), g.adj[i]...)
	}
	return adj
}
