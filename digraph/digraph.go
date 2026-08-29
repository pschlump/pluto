/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package digraph implements a directed graph on the integer vertices
// 0..n-1 with adjacency lists, plus the fundamental digraph algorithms —
// depth-first reachability and paths, breadth-first paths, cycle
// detection, depth-first order, topological sort, and Kosaraju's strong
// components — as constructor-style query objects in the same package
// (Sedgwick, Algorithms, 4th ed., §4.4–4.6).  The vertices ARE the
// indices, like Sedgwick's Digraph; there is no element type.
//
// Operations:
//
//	NewDigraph(n) — Creates a digraph with n vertices and no edges.	O(n)
//	AddEdge(v, w) — Adds the directed edge v->w.			O(1)
//	V() / Len() — Returns the number of vertices.			O(1)
//	E() — Returns the number of edges.				O(1)
//	OutDegree(v) — Returns the number of edges leaving v.		O(1)
//	InDegree(v) — Returns the number of edges entering v.		O(1)
//	Adj(v) — Range-over-func iterator over the out-neighbors of v.	O(outdegree(v))
//	HasEdge(v, w) — Reports whether the edge v->w exists.		O(outdegree(v))
//	Reverse() — Returns a new digraph with every edge flipped.	O(V+E)
//
// Queries (each an immutable snapshot of the digraph at construction
// time):
//
//	NewDirectedDFS(g, sources...) — Reachability from a set of sources.	O(V+E)
//	NewDFSDirectedPaths(g, s) — Paths from s via depth-first search.	O(V+E)
//	NewBFSDirectedPaths(g, s) — Shortest paths from s via BFS.	O(V+E)
//	NewDirectedCycle(g) — A directed cycle, if one exists.		O(V+E)
//	NewDepthFirstOrder(g) — Preorder, postorder, reverse postorder.	O(V+E)
//	NewTopological(g) — A topological order, if the digraph is a DAG.	O(V+E)
//	NewKosarajuSCC(g) — Strongly connected components (Kosaraju).	O(V+E)
//
// Self-loops and parallel edges are allowed: AddEdge(v, v) counts once in
// E, once in OutDegree(v), and once in InDegree(v).  There is
// deliberately no RemoveEdge — matching Sedgwick's Digraph and keeping
// the surface minimal.
//
// Adj yields out-neighbors in insertion order (deterministic — there is
// no random seed).  The digraph must not be modified while an iterator
// is running.
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
// This implementation is NOT thread safe.  A mutex-guarded version with
// the exact same interface lives alongside it in digraph_ts.
package digraph

import (
	"fmt"
	"iter"
)

// Digraph is a directed graph on the vertices 0..n-1, stored as adjacency
// lists.  The in-degree of every vertex is maintained alongside the
// adjacency lists, so InDegree is O(1).
type Digraph struct {
	n     int
	e     int
	adj   [][]int
	indeg []int
}

// NewDigraph creates a digraph with n vertices (0..n-1) and no edges.
// It panics if n < 1.
// Complexity is O(n).
func NewDigraph(n int) *Digraph {
	if n < 1 {
		panic(fmt.Sprintf("digraph: NewDigraph called with n=%d, need n >= 1", n))
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
		panic("digraph: AddEdge called on a nil digraph")
	}
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
	return g.e
}

// OutDegree returns the number of edges leaving v; ok is false if v is
// out of range.
// Complexity is O(1).
func (g *Digraph) OutDegree(v int) (degree int, ok bool) {
	if g == nil || v < 0 || v >= g.n {
		return 0, false
	}
	return len(g.adj[v]), true
}

// InDegree returns the number of edges entering v; ok is false if v is
// out of range.
// Complexity is O(1).
func (g *Digraph) InDegree(v int) (degree int, ok bool) {
	if g == nil || v < 0 || v >= g.n {
		return 0, false
	}
	return g.indeg[v], true
}

// HasEdge reports whether the directed edge v->w exists.  Out-of-range
// vertices report false.
// Complexity is O(outdegree(v)).
func (g *Digraph) HasEdge(v, w int) bool {
	if g == nil || v < 0 || v >= g.n || w < 0 || w >= g.n {
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
// An out-of-range v (or a nil digraph) yields nothing.  The digraph must
// not be modified while the iterator is running.
// Complexity is O(outdegree(v)).
func (g *Digraph) Adj(v int) iter.Seq[int] {
	if g == nil || v < 0 || v >= g.n {
		return func(func(int) bool) {} // nil/out-of-range iterates as empty
	}
	return func(yield func(int) bool) {
		for _, w := range g.adj[v] {
			if !yield(w) {
				return
			}
		}
	}
}

// Reverse returns a new digraph with the same vertices and every edge
// flipped (v->w becomes w->v).  A nil digraph reverses to nil.  The
// original digraph is not modified; the reverse is an independent copy.
// Complexity is O(V+E).
func (g *Digraph) Reverse() *Digraph {
	if g == nil {
		return nil
	}
	r := &Digraph{n: g.n, e: g.e, adj: make([][]int, g.n), indeg: make([]int, g.n)}
	for v := 0; v < g.n; v++ {
		r.indeg[v] = len(g.adj[v]) // in-degree of the reverse = out-degree here
		for _, w := range g.adj[v] {
			r.adj[w] = append(r.adj[w], v)
		}
	}
	return r
}
