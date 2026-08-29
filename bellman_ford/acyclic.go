/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// AcyclicSP and AcyclicLP: single-source shortest and longest paths in
// edge-weighted DAGs (Sedgwick, Algorithms, 4th ed., §4.4 — the book's
// §4.10 AcyclicSP / AcyclicLP).  Relaxing the vertices in topological
// order solves both problems in O(E+V), with negative edge weights
// allowed.  The topological order is computed internally over the
// weighted digraph — the package is self-contained.

package bellman_ford

import (
	"fmt"
	"math"
	"slices"

	"github.com/pschlump/pluto/dijkstra"
)

// topologicalOrder returns a topological order of the digraph with the
// given adjacency lists (reverse postorder of a DFS), or ok=false when
// the digraph has a cycle.
// Complexity is O(E+V).
func topologicalOrder(adj [][]dijkstra.DirectedEdge) (order []int, ok bool) {
	n := len(adj)
	marked := make([]bool, n)
	onStack := make([]bool, n)
	order = make([]int, 0, n)
	var dfs func(v int) bool // false unwinds the search once a cycle is found
	dfs = func(v int) bool {
		marked[v] = true
		onStack[v] = true
		for _, e := range adj[v] {
			w := e.To
			if !marked[w] {
				if !dfs(w) {
					return false
				}
			} else if onStack[w] {
				return false // back edge: the digraph has a cycle
			}
		}
		onStack[v] = false
		order = append(order, v) // postorder; reversed below
		return true
	}
	for v := range n {
		if !marked[v] && !dfs(v) {
			return nil, false
		}
	}
	slices.Reverse(order)
	return order, true
}

// snapshotDigraph copies the adjacency lists of g so a query object is
// an immutable snapshot (dijkstra's Adj returns the live slice).
func snapshotDigraph(g *dijkstra.EdgeWeightedDigraph) [][]dijkstra.DirectedEdge {
	n := g.V()
	adj := make([][]dijkstra.DirectedEdge, n)
	for v := range n {
		adj[v] = append([]dijkstra.DirectedEdge(nil), g.Adj(v)...)
	}
	return adj
}

// -------------------------------------------------------------------------------------------------------
// AcyclicSP — shortest paths in an edge-weighted DAG
// -------------------------------------------------------------------------------------------------------

// AcyclicSP answers shortest-path queries from a source vertex s in an
// edge-weighted DAG by relaxing the vertices in topological order
// (Sedgwick's AcyclicSP).  Negative edge weights are allowed.  It is an
// immutable snapshot of the graph at construction time: later AddEdge
// calls on the graph are not reflected.
type AcyclicSP struct {
	distTo  []float64               // distTo[v] is the length of a shortest s -> v path (+Inf if none)
	edgeTo  []dijkstra.DirectedEdge // edgeTo[v] is the last edge on a shortest s -> v path
	hasEdge []bool                  // hasEdge[v] marks whether edgeTo[v] holds a real edge
	s       int
}

// NewAcyclicSP computes shortest paths from s in g, which must be a DAG.
// It panics on a nil or empty graph, on an out-of-range source, or when
// the digraph has a cycle — there is no sane answer for any of those.
// Complexity is O(E+V) time, O(V) space.
func NewAcyclicSP(g *dijkstra.EdgeWeightedDigraph, s int) *AcyclicSP {
	if g == nil || g.V() == 0 {
		panic("bellman_ford: NewAcyclicSP called on a nil or empty graph")
	}
	if s < 0 || s >= g.V() {
		panic(fmt.Sprintf("bellman_ford: NewAcyclicSP called with out-of-range source %d (graph has %d vertices)", s, g.V()))
	}
	adj := snapshotDigraph(g)
	order, ok := topologicalOrder(adj)
	if !ok {
		panic("bellman_ford: NewAcyclicSP called on a digraph with a cycle — shortest paths by topological relaxation require a DAG (use NewBellmanFordSP)")
	}

	n := len(adj)
	sp := &AcyclicSP{
		distTo:  make([]float64, n),
		edgeTo:  make([]dijkstra.DirectedEdge, n),
		hasEdge: make([]bool, n),
		s:       s,
	}
	for v := range sp.distTo {
		sp.distTo[v] = math.Inf(1)
	}
	sp.distTo[s] = 0
	for _, v := range order {
		for _, e := range adj[v] {
			if nd := sp.distTo[v] + e.Weight; nd < sp.distTo[e.To] {
				sp.distTo[e.To] = nd
				sp.edgeTo[e.To] = e
				sp.hasEdge[e.To] = true
			}
		}
	}
	return sp
}

// DistTo returns the length of a shortest path from the source to v, or
// +Inf if v is unreachable or out of range (or the receiver is nil).
// Complexity is O(1).
func (sp *AcyclicSP) DistTo(v int) float64 {
	if sp == nil || v < 0 || v >= len(sp.distTo) {
		return math.Inf(1)
	}
	return sp.distTo[v]
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (sp *AcyclicSP) HasPathTo(v int) bool {
	if sp == nil || v < 0 || v >= len(sp.distTo) {
		return false
	}
	return !math.IsInf(sp.distTo[v], 1)
}

// PathTo returns a shortest path from the source to v as a slice of
// edges ordered source-first, as a fresh slice the caller may mutate.
// It returns nil if there is no path (or v is out of range); the path
// from the source to itself is an empty non-nil slice.  The edge weights
// along the returned path sum to DistTo(v).
// Complexity is O(path length).
func (sp *AcyclicSP) PathTo(v int) []dijkstra.DirectedEdge {
	if !sp.HasPathTo(v) {
		return nil
	}
	path := make([]dijkstra.DirectedEdge, 0)
	for x := v; x != sp.s; x = sp.edgeTo[x].From {
		path = append(path, sp.edgeTo[x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op provided for API compatibility with the thread-safe
// bellman_ford_ts twin.  This implementation is not safe for concurrent
// use.
func (sp *AcyclicSP) Lock() {}

// Unlock is a no-op provided for API compatibility with the thread-safe
// bellman_ford_ts twin.  This implementation is not safe for concurrent
// use.
func (sp *AcyclicSP) Unlock() {}

// -------------------------------------------------------------------------------------------------------
// AcyclicLP — longest paths in an edge-weighted DAG
// -------------------------------------------------------------------------------------------------------

// AcyclicLP answers longest-path queries from a source vertex s in an
// edge-weighted DAG (Sedgwick's AcyclicLP).  The implementation follows
// algs4 directly: distTo is initialized to -Inf and the relax inequality
// is switched (maximize instead of minimize) — equivalent to negating
// every weight, computing shortest paths, and negating back, but done in
// one pass.  It is an immutable snapshot of the graph at construction
// time.
type AcyclicLP struct {
	distTo  []float64               // distTo[v] is the length of a longest s -> v path (-Inf if none)
	edgeTo  []dijkstra.DirectedEdge // edgeTo[v] is the last edge on a longest s -> v path
	hasEdge []bool                  // hasEdge[v] marks whether edgeTo[v] holds a real edge
	s       int
}

// NewAcyclicLP computes longest paths from s in g, which must be a DAG
// (in a digraph with a reachable cycle the longest path is unbounded —
// no sane answer).  It panics on a nil or empty graph, on an
// out-of-range source, or when the digraph has a cycle.
// Complexity is O(E+V) time, O(V) space.
func NewAcyclicLP(g *dijkstra.EdgeWeightedDigraph, s int) *AcyclicLP {
	if g == nil || g.V() == 0 {
		panic("bellman_ford: NewAcyclicLP called on a nil or empty graph")
	}
	if s < 0 || s >= g.V() {
		panic(fmt.Sprintf("bellman_ford: NewAcyclicLP called with out-of-range source %d (graph has %d vertices)", s, g.V()))
	}
	adj := snapshotDigraph(g)
	order, ok := topologicalOrder(adj)
	if !ok {
		panic("bellman_ford: NewAcyclicLP called on a digraph with a cycle — longest paths are unbounded unless the digraph is a DAG")
	}

	n := len(adj)
	lp := &AcyclicLP{
		distTo:  make([]float64, n),
		edgeTo:  make([]dijkstra.DirectedEdge, n),
		hasEdge: make([]bool, n),
		s:       s,
	}
	for v := range lp.distTo {
		lp.distTo[v] = math.Inf(-1)
	}
	lp.distTo[s] = 0
	for _, v := range order {
		for _, e := range adj[v] {
			if nd := lp.distTo[v] + e.Weight; nd > lp.distTo[e.To] { // switch the inequality: maximize
				lp.distTo[e.To] = nd
				lp.edgeTo[e.To] = e
				lp.hasEdge[e.To] = true
			}
		}
	}
	return lp
}

// DistTo returns the length of a longest path from the source to v, or
// -Inf if v is unreachable or out of range (or the receiver is nil) —
// the mirror image of the shortest-path +Inf convention.
// Complexity is O(1).
func (lp *AcyclicLP) DistTo(v int) float64 {
	if lp == nil || v < 0 || v >= len(lp.distTo) {
		return math.Inf(-1)
	}
	return lp.distTo[v]
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (lp *AcyclicLP) HasPathTo(v int) bool {
	if lp == nil || v < 0 || v >= len(lp.distTo) {
		return false
	}
	return !math.IsInf(lp.distTo[v], -1)
}

// PathTo returns a longest path from the source to v as a slice of
// edges ordered source-first, as a fresh slice the caller may mutate.
// It returns nil if there is no path (or v is out of range); the path
// from the source to itself is an empty non-nil slice.  The edge weights
// along the returned path sum to DistTo(v).
// Complexity is O(path length).
func (lp *AcyclicLP) PathTo(v int) []dijkstra.DirectedEdge {
	if !lp.HasPathTo(v) {
		return nil
	}
	path := make([]dijkstra.DirectedEdge, 0)
	for x := v; x != lp.s; x = lp.edgeTo[x].From {
		path = append(path, lp.edgeTo[x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op provided for API compatibility with the thread-safe
// bellman_ford_ts twin.  This implementation is not safe for concurrent
// use.
func (lp *AcyclicLP) Lock() {}

// Unlock is a no-op provided for API compatibility with the thread-safe
// bellman_ford_ts twin.  This implementation is not safe for concurrent
// use.
func (lp *AcyclicLP) Unlock() {}
