/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package mst_ts

import (
	"slices"

	"github.com/pschlump/pluto/dijkstra_ts"
	"github.com/pschlump/pluto/union_find"
)

// BoruvkaMST is the result of running Borůvka's algorithm (Sedgwick's
// BoruvkaMST) on an edge-weighted graph.  It is an immutable snapshot
// of the graph at construction time — later AddEdge calls are not
// reflected — and is safe for concurrent reads.
type BoruvkaMST struct {
	weight float64
	edges  []dijkstra_ts.Edge
}

// NewBoruvkaMST snapshots the graph's adjacency lists (each under the
// graph's read lock), then computes a minimum spanning tree lock-free
// on the snapshot with Borůvka's algorithm: in rounds, every component
// records its cheapest crossing edge and all those merges apply at
// once, tracked with a union-find (pluto/union_find).  Each round at
// least halves the number of components, so there are O(log V) rounds.
//
// On a disconnected graph the result is the minimum spanning FOREST —
// Borůvka never connects two components, so Edges holds V - (number of
// components) edges.
//
// It panics on a nil or empty graph — no spanning tree exists to
// compute.
// Complexity is O(E log V) time plus O(V+E) for the snapshot, O(V)
// space beyond the result.
func NewBoruvkaMST(g *dijkstra_ts.EdgeWeightedGraph) *BoruvkaMST {
	checkGraph("NewBoruvkaMST", g)
	adj := snapshotAdj(g)
	n := len(adj)

	mst := &BoruvkaMST{}
	uf := union_find.NewUnionFind(n)
	cheapest := make([]dijkstra_ts.Edge, n) // cheapest[r]: the cheapest crossing edge of the component rooted at r
	hasCheapest := make([]bool, n)          // its "no edge yet" marker (a zero Edge looks real)
	for {
		// One Borůvka round: scan every vertex's incident edges and keep
		// the cheapest edge leaving each component.  A self-loop is never
		// a crossing edge (its endpoints share a root); the cheapest of
		// parallel edges wins; the scan sees each undirected edge from
		// both endpoints, which is harmless — both sightings compete for
		// their own component's minimum.
		for v := 0; v < n; v++ {
			r, _ := uf.Find(v)
			for _, e := range adj[v] {
				if rw, _ := uf.Find(e.Other(v)); rw != r {
					// a crossing edge: it leaves v's component
					if !hasCheapest[r] || e.Weight < cheapest[r].Weight {
						cheapest[r] = e
						hasCheapest[r] = true
					}
				}
			}
		}
		// Apply all merges of the round AFTER the scan: unioning during
		// it would leave cheapest indexed by roots the unions just
		// destroyed.  Two components may have picked each other via the
		// same or different edges — the second Union then reports false
		// and the duplicate is skipped, so no edge is counted twice and
		// no cycle can close.
		merged := false
		for r := range cheapest {
			if !hasCheapest[r] {
				continue
			}
			hasCheapest[r] = false
			e := cheapest[r]
			if uf.Union(e.V, e.W) {
				mst.edges = append(mst.edges, e)
				mst.weight += e.Weight
				merged = true
			}
		}
		if !merged {
			break // no crossing edge anywhere: every component is spanned (a forest input, or done)
		}
	}
	return mst
}

// Edges returns the edges of the minimum spanning tree (or forest, on a
// disconnected graph) in the order the algorithm added them — rounds of
// parallel merges, so not sorted by weight (unlike KruskalMST) — as a
// fresh slice the caller may mutate.  A nil receiver reports nil.
// Complexity is O(Len).
func (m *BoruvkaMST) Edges() []dijkstra_ts.Edge {
	if m == nil {
		return nil
	}
	return slices.Clone(m.edges)
}

// Weight returns the total weight of the tree (or forest).  A nil
// receiver reports 0.
// Complexity is O(1).
func (m *BoruvkaMST) Weight() float64 {
	if m == nil {
		return 0
	}
	return m.weight
}

// Len returns the number of tree edges: V-1 on a connected graph, fewer
// for a spanning forest.  A nil receiver reports 0.
// Complexity is O(1).
func (m *BoruvkaMST) Len() int {
	if m == nil {
		return 0
	}
	return len(m.edges)
}
