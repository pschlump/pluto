/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package floyd_warshall implements the Floyd-Warshall all-pairs
// shortest-path algorithm (Sedgwick, Algorithms, 4th ed., §4.4) over the
// edge-weighted digraphs of pluto/dijkstra.  Negative edge weights are
// legal — that is the point of the package — and the algorithm detects a
// negative cycle ANYWHERE in the graph, unlike pluto/bellman_ford's
// source-rooted search which only sees cycles reachable from the source.
//
// NewFloydWarshall(g) is the dense-graph alternative to dijkstra's
// NewDijkstraAllPairsSP: O(V³) regardless of E versus O(V·E log V), and
// the only one of the two that tolerates negative weights.  On dense
// graphs (E close to V²) Floyd-Warshall is the faster choice; on sparse
// ones run V Dijkstras instead.  The price is the O(V²) distance and
// predecessor matrices.
//
//	Dist(v, w) / HasPath(v, w) / Path(v, w) — the query surface,
//	matching dijkstra's all-pairs conventions: Dist is +Inf when w is
//	unreachable, Path returns a source-first slice of edges, nil when
//	there is no path, empty non-nil for a vertex to itself.
//	HasNegativeCycle() / NegativeCycle() — the negative-cycle surface,
//	matching bellman_ford's conventions.
//
// When a negative cycle exists, shortest paths are undefined (Sedgwick:
// Dist/HasPath/Path throw UnsupportedOperationException); here they
// PANIC with a message naming the method and the negative cycle.
//
// A nil *FloydWarshall and the zero value behave as "no paths": Dist
// reports +Inf, HasPath false, Path nil, HasNegativeCycle false,
// NegativeCycle nil.
//
// Panic contract (each panic message names the method and the fix):
// NewFloydWarshall on a nil or empty graph, or when any edge has a
// non-finite weight (NaN or ±Inf — negative weights are legal, degenerate
// floats are not); Dist/HasPath/Path on a FloydWarshall that found a
// negative cycle.  Every other operation tolerates nil and zero-value
// receivers.
//
// This implementation is NOT safe for concurrent use; the mutex-guarded
// twin floyd_warshall_ts has the identical interface.
package floyd_warshall

import (
	"fmt"
	"math"
	"slices"

	"github.com/pschlump/pluto/dijkstra"
)

// FloydWarshall answers shortest-path queries between every pair of
// vertices in an edge-weighted digraph that may have negative edge
// weights (Sedgwick's FloydWarshall, over the adjacency-matrix
// formulation of the dynamic program).  It is an immutable snapshot of
// the graph at construction time: later AddEdge calls on the graph are
// not reflected.
type FloydWarshall struct {
	dist     [][]float64               // dist[v][w] is the length of a shortest v -> w path (+Inf if none)
	edgeTo   [][]dijkstra.DirectedEdge // edgeTo[v][w] is the last edge on a shortest v -> w path (zero value = none)
	negCycle []dijkstra.DirectedEdge   // a negative cycle, nil when there is none
}

// NewFloydWarshall computes shortest paths between every pair of
// vertices in g.  Negative edge weights are legal — that is the point of
// the algorithm.  Parallel edges collapse to the minimum-weight one per
// vertex pair (Sedgwick's adjacency-matrix input cannot even represent
// parallels), and a non-negative self-loop never beats the diagonal 0.
// It panics on a nil or empty graph, or when any edge has a non-finite
// weight (NaN or ±Inf) — programmer errors with no sane answer.
// Complexity is O(V³) time, O(V²) space.
func NewFloydWarshall(g *dijkstra.EdgeWeightedDigraph) *FloydWarshall {
	if g == nil || g.V() == 0 {
		panic("floyd_warshall: NewFloydWarshall called on a nil or empty graph")
	}
	// Snapshot the adjacency lists so the result is immutable even if the
	// graph is mutated later (dijkstra's Adj returns the live slice).
	adj := snapshotDigraph(g)
	n := len(adj)
	fw := &FloydWarshall{
		dist:   make([][]float64, n),
		edgeTo: make([][]dijkstra.DirectedEdge, n),
	}
	for v := range n {
		fw.dist[v] = make([]float64, n)
		fw.edgeTo[v] = make([]dijkstra.DirectedEdge, n)
		for w := range n {
			fw.dist[v][w] = math.Inf(1)
		}
		fw.dist[v][v] = 0
	}
	for v := range n {
		for _, e := range adj[v] {
			if math.IsNaN(e.Weight) || math.IsInf(e.Weight, 0) {
				panic(fmt.Sprintf("floyd_warshall: NewFloydWarshall: edge %d->%d has non-finite weight %g (negative weights are legal, NaN and ±Inf are not)", e.From, e.To, e.Weight))
			}
			// The strict < keeps the minimum among parallel edges and
			// leaves the diagonal at 0 unless a NEGATIVE self-loop beats
			// it — exactly algs4's "in case of self-loops" reset,
			// generalized to adjacency-list input.
			if e.Weight < fw.dist[v][e.To] {
				fw.dist[v][e.To] = e.Weight
				fw.edgeTo[v][e.To] = e
			}
		}
	}
	for i := range n {
		// Compute shortest paths using only 0, 1, ..., i as intermediate
		// vertices: one row update per (i, v) pair.
		for v := range n {
			if math.IsInf(fw.dist[v][i], 1) {
				// v cannot reach i even via 0..i — nothing to relax
				// through i (algs4's null edgeTo check; processing i == v
				// would be a no-op: 0 + d < d is false).
				continue
			}
			dvi := fw.dist[v][i]
			rowV, rowI := fw.dist[v], fw.dist[i]
			evRow, eeRow := fw.edgeTo[v], fw.edgeTo[i]
			for w := range n {
				if nd := dvi + rowI[w]; nd < rowV[w] {
					rowV[w] = nd
					evRow[w] = eeRow[w] // rowI[w] is finite here, so this is a real edge
				}
			}
			// A negative diagonal entry means v lies on a cycle of
			// negative total weight: the distances are meaningless, so
			// stop now (algs4 returns early; the frozen state is exactly
			// what the cycle reconstruction below needs).
			if fw.dist[v][v] < 0 {
				fw.negCycle = findNegativeCycle(fw.edgeTo[v], v)
				return fw
			}
		}
	}
	return fw
}

// snapshotDigraph copies the adjacency lists of g so that the computed
// matrices are immutable.
func snapshotDigraph(g *dijkstra.EdgeWeightedDigraph) [][]dijkstra.DirectedEdge {
	adj := make([][]dijkstra.DirectedEdge, g.V())
	for v := range adj {
		adj[v] = slices.Clone(g.Adj(v))
	}
	return adj
}

// findNegativeCycle extracts a negative cycle from the predecessor row
// of v — the vertex whose diagonal went negative — and returns it in
// cycle order (each edge's From is the previous edge's To, and the last
// edge's To is the first edge's From).  The predecessor walk from v
// cannot die out: every vertex it reaches is reachable from v and so has
// a predecessor edge, and it cannot stop at v because the negative
// diagonal means v is on the walk's cycle.  It therefore revisits a
// vertex within n steps; the walk segment between the two visits is the
// cycle.  (algs4 walks the same predecessor row looking for a vertex
// that lies on the cycle; stamping the walk handles the rho-shaped tail
// exactly.)
// Complexity is O(V).
func findNegativeCycle(wp []dijkstra.DirectedEdge, v int) []dijkstra.DirectedEdge {
	n := len(wp)
	stamp := make([]int, n) // stamp[x] is x's position in the walk, -1 when unseen
	for i := range stamp {
		stamp[i] = -1
	}
	walk := make([]int, 0, n)
	x := v
	for stamp[x] < 0 {
		stamp[x] = len(walk)
		walk = append(walk, x)
		x = wp[x].From
	}
	entry := walk[stamp[x]]
	rev := make([]dijkstra.DirectedEdge, 0, len(walk)-stamp[x])
	for y := entry; ; {
		rev = append(rev, wp[y])
		y = wp[y].From
		if y == entry {
			break
		}
	}
	slices.Reverse(rev)
	return rev
}

// HasNegativeCycle reports whether the graph has a negative cycle
// (anywhere in it — not merely reachable from some source, unlike
// bellman_ford).  When it does, shortest paths are undefined: Dist,
// HasPath and Path panic.
// Complexity is O(1).
func (fw *FloydWarshall) HasNegativeCycle() bool {
	return fw != nil && fw.negCycle != nil
}

// NegativeCycle returns a negative cycle as a slice of edges in cycle
// order (each edge's From is the previous edge's To, and the last edge's
// To is the first edge's From), as a fresh slice the caller may mutate.
// It returns nil when there is no negative cycle.
// Complexity is O(cycle length).
func (fw *FloydWarshall) NegativeCycle() []dijkstra.DirectedEdge {
	if fw == nil || fw.negCycle == nil {
		return nil
	}
	return slices.Clone(fw.negCycle)
}

// Dist returns the length of a shortest path from v to w, or +Inf if w
// is unreachable from v or either vertex is out of range (or the
// receiver is nil or the zero value).
// It panics when the constructor found a negative cycle — then shortest
// paths are undefined (algs4 throws UnsupportedOperationException; pluto
// panics).
// Complexity is O(1).
func (fw *FloydWarshall) Dist(v, w int) float64 {
	if fw == nil || v < 0 || v >= len(fw.dist) || w < 0 || w >= len(fw.dist) {
		return math.Inf(1)
	}
	if fw.negCycle != nil {
		panic(fmt.Sprintf("floyd_warshall: Dist called after a negative cycle was found — shortest paths are undefined (cycle: %v)", fw.negCycle))
	}
	return fw.dist[v][w]
}

// HasPath reports whether w is reachable from v.  Out-of-range vertices
// (and a nil or zero-value receiver) report false.
// It panics when the constructor found a negative cycle — then shortest
// paths are undefined.
// Complexity is O(1).
func (fw *FloydWarshall) HasPath(v, w int) bool {
	if fw == nil || v < 0 || v >= len(fw.dist) || w < 0 || w >= len(fw.dist) {
		return false
	}
	if fw.negCycle != nil {
		panic(fmt.Sprintf("floyd_warshall: HasPath called after a negative cycle was found — shortest paths are undefined (cycle: %v)", fw.negCycle))
	}
	return !math.IsInf(fw.dist[v][w], 1)
}

// Path returns a shortest path from v to w as a slice of edges ordered
// source-first, as a fresh slice the caller may mutate.  It returns nil
// if there is no path (or a vertex is out of range); the path from a
// vertex to itself is an empty non-nil slice.  The edge weights along
// the returned path sum to Dist(v, w).
// It panics when the constructor found a negative cycle — then shortest
// paths are undefined.
// Complexity is O(path length).
func (fw *FloydWarshall) Path(v, w int) []dijkstra.DirectedEdge {
	if fw != nil && fw.negCycle != nil {
		panic(fmt.Sprintf("floyd_warshall: Path called after a negative cycle was found — shortest paths are undefined (cycle: %v)", fw.negCycle))
	}
	if !fw.HasPath(v, w) {
		return nil
	}
	path := make([]dijkstra.DirectedEdge, 0)
	for x := w; x != v; x = fw.edgeTo[v][x].From {
		path = append(path, fw.edgeTo[v][x])
	}
	slices.Reverse(path)
	return path
}

// Lock is a no-op provided for API compatibility with the thread-safe
// floyd_warshall_ts twin.  This implementation is not safe for
// concurrent use.
func (fw *FloydWarshall) Lock() {}

// Unlock is a no-op provided for API compatibility with the thread-safe
// floyd_warshall_ts twin.  This implementation is not safe for
// concurrent use.
func (fw *FloydWarshall) Unlock() {}
