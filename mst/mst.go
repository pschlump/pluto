/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package mst implements the three classic minimum-spanning-tree
// algorithms of Sedgwick & Wayne, Algorithms, 4th ed. §4.3, as
// constructor-style query objects over pluto/dijkstra's
// EdgeWeightedGraph — an intra-pluto composition, like dijkstra over
// index_pq: the graph container is exactly what an MST algorithm
// consumes.
//
//	NewLazyPrimMST(g) — lazy Prim's algorithm, on pluto/priority_queue.	O(E log E)
//	NewPrimMST(g)     — eager Prim's algorithm, on pluto/index_pq.	O(E log V)
//	NewKruskalMST(g)  — Kruskal's algorithm, on pluto/union_find.	O(E log E)
//
// All three result types share one surface: Edges returns the tree
// edges, Weight their total weight, Len their count.  The result is
// computed eagerly in the constructor, so each query object is an
// immutable snapshot of the graph at construction time — later AddEdge
// calls are not reflected.
//
// A minimum spanning tree exists only for a connected graph; on a
// disconnected graph every constructor produces the minimum spanning
// FOREST — one minimum tree per connected component — and Edges then
// holds fewer than V-1 edges.
//
// Edge weights may be negative: an MST is well-defined for arbitrary
// weights, and unlike the Dijkstra constructors nothing here validates
// or rejects them.  Self-loops and parallel edges (both allowed by the
// graph) are handled: a self-loop can never join a spanning tree and
// the cheapest of a set of parallel edges wins.
//
// Panic contract: each constructor panics on a nil or empty graph
// (V == 0) — no spanning tree exists to compute.  That is the package's
// only panic.  A nil result object tolerates every read: Edges reports
// nil, Weight 0 and Len 0.
//
// This implementation is NOT safe for concurrent use; the mutex-guarded
// twin mst_ts has the identical interface over dijkstra_ts graphs.
package mst

import (
	"fmt"

	"github.com/pschlump/pluto/dijkstra"
)

// compareEdgeByWeight orders two edges by weight, ascending.  It backs
// both the lazy-Prim priority queue and Kruskal's sort.
func compareEdgeByWeight(a, b dijkstra.Edge) int {
	switch {
	case a.Weight < b.Weight:
		return -1
	case a.Weight > b.Weight:
		return 1
	default:
		return 0
	}
}

// checkGraph enforces the panic contract shared by the three
// constructors; who names the constructor for the panic message.
func checkGraph(who string, g *dijkstra.EdgeWeightedGraph) {
	if g == nil || g.V() == 0 {
		panic(fmt.Sprintf("mst: %s called on a nil or empty graph (no spanning tree exists to compute)", who))
	}
}
