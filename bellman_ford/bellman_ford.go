/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bellman_ford implements the queue-based Bellman-Ford
// shortest-path algorithm with negative-cycle detection (Sedgwick,
// Algorithms, 4th ed., §4.4 — the book's §4.11 BellmanFordSP) over the
// edge-weighted digraphs of pluto/dijkstra, plus the §4.10 linear-time
// shortest/longest-path algorithms for edge-weighted DAGs (AcyclicSP /
// AcyclicLP in acyclic.go).  Unlike Dijkstra's algorithm, negative edge
// weights are legal — that is the point of the package.
//
// The queries are constructor-style immutable snapshots of the graph at
// construction time, exactly like pluto/dijkstra's query objects:
//
//	NewBellmanFordSP(g, s) — shortest paths from s, or a reachable
//	                         negative cycle.	O(E·V) worst case
//	NewAcyclicSP(g, s) — shortest paths from s in a DAG.	O(E+V)
//	NewAcyclicLP(g, s) — longest paths from s in a DAG.	O(E+V)
//
//	DistTo(v) / HasPathTo(v) / PathTo(v) — the query surface, matching
//	dijkstra's conventions: DistTo is +Inf when v is unreachable,
//	PathTo returns a source-first slice of edges, nil when there is no
//	path, empty non-nil for the source-to-itself path.
//	HasNegativeCycle() / NegativeCycle() — BellmanFordSP only.
//
// When a negative cycle is reachable from the source, shortest paths are
// undefined (Sedgwick: DistTo/PathTo throw UnsupportedOperationException);
// here DistTo and PathTo PANIC with a message naming the method and the
// negative cycle.
//
// A nil *BellmanFordSP / *AcyclicSP / *AcyclicLP behaves as "no paths":
// DistTo reports +Inf, HasPathTo false, PathTo nil, HasNegativeCycle
// false, NegativeCycle nil.
//
// Panic contract (each panic message names the method and the fix):
// NewBellmanFordSP/NewAcyclicSP/NewAcyclicLP on a nil or empty graph or
// with an out-of-range source; NewAcyclicSP/NewAcyclicLP when the digraph
// has a cycle; DistTo/PathTo on a BellmanFordSP that found a negative
// cycle.  Every other operation tolerates nil receivers.
//
// This implementation is NOT safe for concurrent use; the mutex-guarded
// twin bellman_ford_ts has the identical interface.
package bellman_ford

import (
	"fmt"
	"math"
	"slices"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/queue"
)

// BellmanFordSP answers shortest-path queries from a source vertex s in
// an edge-weighted digraph that may have negative edge weights
// (Sedgwick's queue-based BellmanFordSP).  It is an immutable snapshot
// of the graph at construction time: later AddEdge calls on the graph
// are not reflected.
type BellmanFordSP struct {
	distTo  []float64               // distTo[v] is the length of a shortest s -> v path (+Inf if none)
	edgeTo  []dijkstra.DirectedEdge // edgeTo[v] is the last edge on a shortest s -> v path
	hasEdge []bool                  // hasEdge[v] marks whether edgeTo[v] holds a real edge
	s       int
	cycle   []dijkstra.DirectedEdge // a negative cycle reachable from s, nil when there is none
}

// NewBellmanFordSP computes shortest paths from s in g, or finds a
// negative cycle reachable from s.  Negative edge weights are legal —
// that is the point of the algorithm.  It panics on a nil or empty graph
// or on an out-of-range source — programmer errors with no sane answer.
// Complexity is O(E·V) time in the worst case (typically much less),
// O(V) space.
func NewBellmanFordSP(g *dijkstra.EdgeWeightedDigraph, s int) *BellmanFordSP {
	if g == nil || g.V() == 0 {
		panic("bellman_ford: NewBellmanFordSP called on a nil or empty graph")
	}
	if s < 0 || s >= g.V() {
		panic(fmt.Sprintf("bellman_ford: NewBellmanFordSP called with out-of-range source %d (graph has %d vertices)", s, g.V()))
	}
	// Snapshot the adjacency lists so the result is immutable even if the
	// graph is mutated later (dijkstra's Adj returns the live slice).
	return bellmanFordFrom(snapshotDigraph(g), s)
}

// bellmanFordFrom runs the queue-based Bellman-Ford algorithm from s
// over adj; the lock-free core, mirrored by the bellman_ford_ts twin.
// Complexity is O(E·V) time in the worst case, O(V) space.
func bellmanFordFrom(adj [][]dijkstra.DirectedEdge, s int) *BellmanFordSP {
	n := len(adj)
	sp := &BellmanFordSP{
		distTo:  make([]float64, n),
		edgeTo:  make([]dijkstra.DirectedEdge, n),
		hasEdge: make([]bool, n),
		s:       s,
	}
	for v := range sp.distTo {
		sp.distTo[v] = math.Inf(1)
	}
	sp.distTo[s] = 0

	// The FIFO holds the vertices whose distTo decreased and whose
	// outgoing edges therefore need (re-)relaxation; onQueue avoids
	// duplicates (an intra-pluto composition on pluto/queue, like
	// graph's BFS).
	var q queue.Queue[int]
	onQueue := make([]bool, n)
	q.Push(s)
	onQueue[s] = true
	relaxRounds := 0 // amortize the negative-cycle check: once every n vertex relaxations
	for !q.IsEmpty() && sp.cycle == nil {
		v, _ := q.Dequeue()
		onQueue[v] = false
		for _, e := range adj[v] {
			w := e.To
			if nd := sp.distTo[v] + e.Weight; nd < sp.distTo[w] {
				sp.distTo[w] = nd
				sp.edgeTo[w] = e
				sp.hasEdge[w] = true
				if !onQueue[w] {
					q.Push(w)
					onQueue[w] = true
				}
			}
		}
		if relaxRounds++; relaxRounds%n == 0 {
			// If the queue has not emptied after n full rounds of
			// relaxations, the edgeTo subgraph contains a negative cycle.
			sp.cycle = findNegativeCycle(sp.edgeTo, sp.hasEdge)
		}
	}
	if sp.cycle == nil {
		sp.cycle = findNegativeCycle(sp.edgeTo, sp.hasEdge) // belt and braces: a final check
	}
	return sp
}

// findNegativeCycle looks for a directed cycle in the subgraph formed by
// the edgeTo edges (the shortest-path-tree candidate) and returns it in
// cycle order — each edge's From is the previous edge's To and the last
// edge's To is the first edge's From — or nil when the subgraph is
// acyclic.  (Sedgwick's EdgeWeightedDirectedCycle over the edgeTo
// subgraph.)
func findNegativeCycle(edgeTo []dijkstra.DirectedEdge, hasEdge []bool) []dijkstra.DirectedEdge {
	n := len(edgeTo)
	// Adjacency of the edgeTo subgraph, keyed by tail vertex.
	sptAdj := make([][]dijkstra.DirectedEdge, n)
	for w := range n {
		if hasEdge[w] {
			e := edgeTo[w]
			sptAdj[e.From] = append(sptAdj[e.From], e)
		}
	}
	marked := make([]bool, n)
	onStack := make([]bool, n)
	dfsEdgeTo := make([]dijkstra.DirectedEdge, n) // DFS tree edges, for walking back to the cycle entry
	var cycle []dijkstra.DirectedEdge
	var dfs func(v int)
	dfs = func(v int) {
		marked[v] = true
		onStack[v] = true
		for _, e := range sptAdj[v] {
			if cycle != nil {
				return
			}
			w := e.To
			if !marked[w] {
				dfsEdgeTo[w] = e
				dfs(w)
			} else if onStack[w] {
				// Back edge v -> w: walk the DFS tree from v back to w to
				// collect the cycle, then reverse into cycle order.
				rev := []dijkstra.DirectedEdge{e}
				for f := e; f.From != w; {
					f = dfsEdgeTo[f.From]
					rev = append(rev, f)
				}
				slices.Reverse(rev)
				cycle = rev
				return
			}
		}
		onStack[v] = false
	}
	for v := range n {
		if !marked[v] && cycle == nil {
			dfs(v)
		}
	}
	return cycle
}

// HasNegativeCycle reports whether a negative cycle is reachable from
// the source.  When it is, shortest paths are undefined: DistTo and
// PathTo panic.
// Complexity is O(1).
func (sp *BellmanFordSP) HasNegativeCycle() bool {
	return sp != nil && sp.cycle != nil
}

// NegativeCycle returns a negative cycle reachable from the source as a
// slice of edges in cycle order (each edge's From is the previous edge's
// To, and the last edge's To is the first edge's From), as a fresh slice
// the caller may mutate.  It returns nil when there is no negative
// cycle.
// Complexity is O(cycle length).
func (sp *BellmanFordSP) NegativeCycle() []dijkstra.DirectedEdge {
	if sp == nil || sp.cycle == nil {
		return nil
	}
	return slices.Clone(sp.cycle)
}

// DistTo returns the length of a shortest path from the source to v, or
// +Inf if v is unreachable or out of range (or the receiver is nil).
// It panics when the constructor found a negative cycle reachable from
// the source — then shortest paths are undefined (algs4 throws
// UnsupportedOperationException; pluto panics).
// Complexity is O(1).
func (sp *BellmanFordSP) DistTo(v int) float64 {
	if sp == nil || v < 0 || v >= len(sp.distTo) {
		return math.Inf(1)
	}
	if sp.cycle != nil {
		panic(fmt.Sprintf("bellman_ford: DistTo called after a negative cycle was found — shortest paths are undefined (cycle: %v)", sp.cycle))
	}
	return sp.distTo[v]
}

// HasPathTo reports whether there is a path from the source to v.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (sp *BellmanFordSP) HasPathTo(v int) bool {
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
// It panics when the constructor found a negative cycle reachable from
// the source — then shortest paths are undefined.
// Complexity is O(path length).
func (sp *BellmanFordSP) PathTo(v int) []dijkstra.DirectedEdge {
	if sp != nil && sp.cycle != nil {
		panic(fmt.Sprintf("bellman_ford: PathTo called after a negative cycle was found — shortest paths are undefined (cycle: %v)", sp.cycle))
	}
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
func (sp *BellmanFordSP) Lock() {}

// Unlock is a no-op provided for API compatibility with the thread-safe
// bellman_ford_ts twin.  This implementation is not safe for concurrent
// use.
func (sp *BellmanFordSP) Unlock() {}
