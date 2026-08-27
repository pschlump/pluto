/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package graph implements an undirected graph on the integer vertices
// 0..n-1 with adjacency lists, plus the three fundamental graph queries —
// depth-first paths, breadth-first paths, and connected components — as
// constructor-style query objects in the same package (Sedgwick, Algorithms,
// 4th ed., §4.1).  The vertices ARE the indices, like Sedgwick's Graph;
// there is no element type.
//
// Operations:
//
//	NewGraph(n) — Creates a graph with n vertices and no edges.		O(n)
//	AddEdge(v, w) — Adds the edge v-w.					O(1)
//	V() / Len() — Returns the number of vertices.				O(1)
//	E() — Returns the number of edges.					O(1)
//	Degree(v) — Returns the number of edges incident to v.			O(1)
//	Adj(v) — Range-over-func iterator over the neighbors of v.		O(degree(v))
//	HasEdge(v, w) — Reports whether the edge v-w exists.			O(degree(v))
//
// Queries (each an immutable snapshot of the graph at construction time):
//
//	NewDFSPaths(g, s) — Paths from s via depth-first search.		O(V+E)
//	NewBFSPaths(g, s) — Shortest paths from s via breadth-first search.	O(V+E)
//	NewCC(g) — Connected components.					O(V+E)
//
// Self-loops and parallel edges are allowed: AddEdge(v, v) counts once in
// E and twice in Degree(v), exactly like Sedgwick.  There is deliberately
// no RemoveEdge — matching Sedgwick's Graph and keeping the surface
// minimal.
//
// Adj yields neighbors in insertion order (deterministic — there is no
// random seed).  The graph must not be modified while an iterator is
// running.
//
// Panic contract (each panic message names the method and the fix):
// NewGraph with n < 1; AddEdge on a nil *Graph; NewDFSPaths/NewBFSPaths on
// a nil or empty graph or with an out-of-range source.  Every other
// operation tolerates a nil or zero-value graph as an empty graph with no
// vertices (a zero-value graph reports all vertices out of range, so
// AddEdge on it returns false).
//
// This implementation is NOT thread safe.  A mutex-guarded version with
// the exact same interface lives alongside it in graph_ts.
package graph

import (
	"fmt"
	"iter"
)

// Graph is an undirected graph on the vertices 0..n-1, stored as adjacency
// lists.
type Graph struct {
	n   int
	e   int
	adj [][]int
}

// NewGraph creates a graph with n vertices (0..n-1) and no edges.
// It panics if n < 1.
// Complexity is O(n).
func NewGraph(n int) *Graph {
	if n < 1 {
		panic(fmt.Sprintf("graph: NewGraph called with n=%d, need n >= 1", n))
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
		panic("graph: AddEdge called on a nil graph")
	}
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
	return g.e
}

// Degree returns the number of edges incident to v; ok is false if v is
// out of range (a self-loop counts twice).
// Complexity is O(1).
func (g *Graph) Degree(v int) (degree int, ok bool) {
	if g == nil || v < 0 || v >= g.n {
		return 0, false
	}
	return len(g.adj[v]), true
}

// HasEdge reports whether the edge v-w exists.  Out-of-range vertices
// report false.
// Complexity is O(degree(v)).
func (g *Graph) HasEdge(v, w int) bool {
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

// Adj returns a range-over-func iterator that yields the neighbors of v in
// insertion order.
//
//	for w := range g.Adj(v) {
//		...
//	}
//
// An out-of-range v (or a nil graph) yields nothing.  The graph must not
// be modified while the iterator is running.
// Complexity is O(degree(v)).
func (g *Graph) Adj(v int) iter.Seq[int] {
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
