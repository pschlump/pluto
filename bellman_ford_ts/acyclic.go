/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// AcyclicSP and AcyclicLP: single-source shortest and longest paths in
// edge-weighted DAGs (Sedgwick, Algorithms, 4th ed., §4.4 — the book's
// §4.10 AcyclicSP / AcyclicLP), the thread-safe twins over
// pluto/dijkstra_ts graphs.  The constructors snapshot the adjacency
// under the graph's read lock and then compute lock-free.

package bellman_ford_ts

import (
	"fmt"
	"math"
	"slices"

	"github.com/pschlump/pluto/dijkstra_ts"
)

// topologicalOrder returns a topological order of the digraph with the
// given adjacency lists (reverse postorder of a DFS), or ok=false when
// the digraph has a cycle.
// Complexity is O(E+V).
func topologicalOrder(adj [][]dijkstra_ts.DirectedEdge) (order []int, ok bool) {
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

// snapshotDigraph copies the adjacency lists of g; dijkstra_ts's Adj
// returns a snapshot copy taken under the read lock, so each vertex's
// list is consistent — under concurrent AddEdge calls different
// vertices' lists may reflect slightly different points in time.
func snapshotDigraph(g *dijkstra_ts.EdgeWeightedDigraph) [][]dijkstra_ts.DirectedEdge {
	n := g.V()
	adj := make([][]dijkstra_ts.DirectedEdge, n)
	for v := range n {
		adj[v] = g.Adj(v) // per-vertex snapshot under the read lock
	}
	return adj
}

// -------------------------------------------------------------------------------------------------------
// AcyclicSP — shortest paths in an edge-weighted DAG
// -------------------------------------------------------------------------------------------------------

// AcyclicSP answers shortest-path queries from a source vertex s in an
// edge-weighted DAG by relaxing the vertices in topological order
// (Sedgwick's AcyclicSP).  Negative edge weights are allowed.  It is an
// immutable snapshot of the graph at construction time and is safe for
// concurrent reads.
type AcyclicSP struct {
	distTo  []float64                  // distTo[v] is the length of a shortest s -> v path (+Inf if none)
	edgeTo  []dijkstra_ts.DirectedEdge // edgeTo[v] is the last edge on a shortest s -> v path
	hasEdge []bool                     // hasEdge[v] marks whether edgeTo[v] holds a real edge
	s       int
}

// NewAcyclicSP snapshots the graph's adjacency lists under the read
// lock, then computes shortest paths from s lock-free on the snapshot;
// the snapshot must be a DAG.  It panics on a nil or empty graph, on an
// out-of-range source, or when the digraph has a cycle — there is no
// sane answer for any of those.
// Complexity is O(E+V) time, O(V) space.
func NewAcyclicSP(g *dijkstra_ts.EdgeWeightedDigraph, s int) *AcyclicSP {
	if g == nil {
		panic("bellman_ford_ts: NewAcyclicSP called on a nil graph")
	}
	n := g.V()
	if n == 0 {
		panic("bellman_ford_ts: NewAcyclicSP called on an empty graph")
	}
	if s < 0 || s >= n {
		panic(fmt.Sprintf("bellman_ford_ts: NewAcyclicSP called with out-of-range source %d (graph has %d vertices)", s, n))
	}
	adj := snapshotDigraph(g)
	order, ok := topologicalOrder(adj)
	if !ok {
		panic("bellman_ford_ts: NewAcyclicSP called on a digraph with a cycle — shortest paths by topological relaxation require a DAG (use NewBellmanFordSP)")
	}

	sp := &AcyclicSP{
		distTo:  make([]float64, n),
		edgeTo:  make([]dijkstra_ts.DirectedEdge, n),
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
func (sp *AcyclicSP) PathTo(v int) []dijkstra_ts.DirectedEdge {
	if !sp.HasPathTo(v) {
		return nil
	}
	path := make([]dijkstra_ts.DirectedEdge, 0)
	for x := v; x != sp.s; x = sp.edgeTo[x].From {
		path = append(path, sp.edgeTo[x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op: an AcyclicSP is immutable after construction, so
// there is nothing to lock.  Kept for API parity with the bellman_ford
// package.
func (sp *AcyclicSP) Lock() {}

// Unlock is a no-op: an AcyclicSP is immutable after construction, so
// there is nothing to lock.  Kept for API parity with the bellman_ford
// package.
func (sp *AcyclicSP) Unlock() {}

// -------------------------------------------------------------------------------------------------------
// AcyclicLP — longest paths in an edge-weighted DAG
// -------------------------------------------------------------------------------------------------------

// AcyclicLP answers longest-path queries from a source vertex s in an
// edge-weighted DAG (Sedgwick's AcyclicLP).  The implementation follows
// algs4 directly: distTo is initialized to -Inf and the relax inequality
// is switched (maximize instead of minimize).  It is an immutable
// snapshot of the graph at construction time and is safe for concurrent
// reads.
type AcyclicLP struct {
	distTo  []float64                  // distTo[v] is the length of a longest s -> v path (-Inf if none)
	edgeTo  []dijkstra_ts.DirectedEdge // edgeTo[v] is the last edge on a longest s -> v path
	hasEdge []bool                     // hasEdge[v] marks whether edgeTo[v] holds a real edge
	s       int
}

// NewAcyclicLP snapshots the graph's adjacency lists under the read
// lock, then computes longest paths from s lock-free on the snapshot;
// the snapshot must be a DAG (in a digraph with a reachable cycle the
// longest path is unbounded — no sane answer).  It panics on a nil or
// empty graph, on an out-of-range source, or when the digraph has a
// cycle.
// Complexity is O(E+V) time, O(V) space.
func NewAcyclicLP(g *dijkstra_ts.EdgeWeightedDigraph, s int) *AcyclicLP {
	if g == nil {
		panic("bellman_ford_ts: NewAcyclicLP called on a nil graph")
	}
	n := g.V()
	if n == 0 {
		panic("bellman_ford_ts: NewAcyclicLP called on an empty graph")
	}
	if s < 0 || s >= n {
		panic(fmt.Sprintf("bellman_ford_ts: NewAcyclicLP called with out-of-range source %d (graph has %d vertices)", s, n))
	}
	adj := snapshotDigraph(g)
	order, ok := topologicalOrder(adj)
	if !ok {
		panic("bellman_ford_ts: NewAcyclicLP called on a digraph with a cycle — longest paths are unbounded unless the digraph is a DAG")
	}

	lp := &AcyclicLP{
		distTo:  make([]float64, n),
		edgeTo:  make([]dijkstra_ts.DirectedEdge, n),
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
func (lp *AcyclicLP) PathTo(v int) []dijkstra_ts.DirectedEdge {
	if !lp.HasPathTo(v) {
		return nil
	}
	path := make([]dijkstra_ts.DirectedEdge, 0)
	for x := v; x != lp.s; x = lp.edgeTo[x].From {
		path = append(path, lp.edgeTo[x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op: an AcyclicLP is immutable after construction, so
// there is nothing to lock.  Kept for API parity with the bellman_ford
// package.
func (lp *AcyclicLP) Lock() {}

// Unlock is a no-op: an AcyclicLP is immutable after construction, so
// there is nothing to lock.  Kept for API parity with the bellman_ford
// package.
func (lp *AcyclicLP) Unlock() {}
