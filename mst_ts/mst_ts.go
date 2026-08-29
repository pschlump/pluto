/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package mst_ts implements the three classic minimum-spanning-tree
// algorithms of Sedgwick & Wayne, Algorithms, 4th ed. §4.3, over
// pluto/dijkstra_ts's EdgeWeightedGraph — the thread-safe twin of
// github.com/pschlump/pluto/mst, with the identical API over the
// mutex-guarded graph.
//
//	NewLazyPrimMST(g) — lazy Prim's algorithm, on pluto/priority_queue.	O(E log E)
//	NewPrimMST(g)     — eager Prim's algorithm, on pluto/index_pq.	O(E log V)
//	NewKruskalMST(g)  — Kruskal's algorithm, on pluto/union_find.	O(E log E)
//
// All three result types share one surface: Edges returns the tree
// edges, Weight their total weight, Len their count.
//
// Concurrency model: each constructor reads the graph's adjacency
// through dijkstra_ts.Adj — one snapshot copy per vertex list, each
// taken under the graph's read lock — and then computes lock-free on
// the snapshots; no lock is held during the computation.  (The snapshot
// is per list, not atomic across the whole graph: an edge added
// concurrently may be seen from one endpoint's list and not the
// other's.  The result is a valid MST of SOME recent state of the
// graph, computed race-free.)  The result objects are immutable after
// construction, so they are safe for concurrent reads.
//
// On a disconnected graph every constructor produces the minimum
// spanning FOREST — one minimum tree per connected component, so Edges
// holds fewer than V-1 edges.  Edge weights may be negative, and
// self-loops and parallel edges are handled (a self-loop never joins a
// tree; the cheapest of parallel edges wins).
//
// Panic contract: each constructor panics on a nil or empty graph
// (V == 0) — no spanning tree exists to compute.  That is the package's
// only panic.  A nil result object tolerates every read: Edges reports
// nil, Weight 0 and Len 0.
//
// Run the tests with -race.
package mst_ts

import (
	"fmt"

	"github.com/pschlump/pluto/dijkstra_ts"
)

// compareEdgeByWeight orders two edges by weight, ascending.  It backs
// both the lazy-Prim priority queue and Kruskal's sort.
func compareEdgeByWeight(a, b dijkstra_ts.Edge) int {
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
func checkGraph(who string, g *dijkstra_ts.EdgeWeightedGraph) {
	if g == nil || g.V() == 0 {
		panic(fmt.Sprintf("mst_ts: %s called on a nil or empty graph (no spanning tree exists to compute)", who))
	}
}

// snapshotAdj returns one snapshot copy per adjacency list; each
// dijkstra_ts.Adj call is itself a snapshot taken under the graph's
// read lock.  The caller computes on the returned slices lock-free.
func snapshotAdj(g *dijkstra_ts.EdgeWeightedGraph) [][]dijkstra_ts.Edge {
	adj := make([][]dijkstra_ts.Edge, g.V())
	for v := range adj {
		adj[v] = g.Adj(v)
	}
	return adj
}
