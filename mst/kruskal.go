/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package mst

import (
	"slices"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/union_find"
)

// KruskalMST is the result of running Kruskal's algorithm (Sedgwick's
// KruskalMST) on an edge-weighted graph.  It is an immutable snapshot
// of the graph at construction time: later AddEdge calls on the graph
// are not reflected.
type KruskalMST struct {
	weight float64
	edges  []dijkstra.Edge
}

// NewKruskalMST computes a minimum spanning tree of g with Kruskal's
// algorithm: sort every edge by weight and greedily add each edge that
// does not close a cycle, tracked with a union-find (pluto/union_find).
//
// On a disconnected graph the result is the minimum spanning FOREST —
// Kruskal never connects two components, so Edges holds V - (number of
// components) edges.
//
// It panics on a nil or empty graph — no spanning tree exists to
// compute.
// Complexity is O(E log E) time, O(E) space.
func NewKruskalMST(g *dijkstra.EdgeWeightedGraph) *KruskalMST {
	checkGraph("NewKruskalMST", g)
	n := g.V()

	// Gather the edges.  An undirected edge sits in BOTH endpoints'
	// adjacency lists, in whichever orientation the caller supplied to
	// AddEdge (the stored Edge is not normalized, so an e.V <= e.W
	// filter would silently drop edges added with V > W — tinyEWG has
	// them, e.g. 6--2).  Instead admit each edge only from its
	// lower-numbered endpoint's list (e.Other(v) > v), which keeps every
	// non-self-loop edge exactly once.  A self-loop is admitted twice —
	// both copies sit in the same list and Other(v) == v passes ">=" —
	// but it can never join a spanning tree: the union-find below
	// reports its endpoints already connected and skips the copies, so
	// the duplicates are deliberately not filtered out.
	var all []dijkstra.Edge
	for v := 0; v < n; v++ {
		for _, e := range g.Adj(v) {
			if e.Other(v) >= v {
				all = append(all, e)
			}
		}
	}
	slices.SortFunc(all, compareEdgeByWeight)

	mst := &KruskalMST{}
	uf := union_find.NewUnionFind(n)
	for _, e := range all {
		if uf.Union(e.V, e.W) { // false when already connected: the edge would close a cycle
			mst.edges = append(mst.edges, e)
			mst.weight += e.Weight
		}
	}
	return mst
}

// Edges returns the edges of the minimum spanning tree (or forest, on a
// disconnected graph) in ascending weight order, as a fresh slice the
// caller may mutate.  A nil receiver reports nil.
// Complexity is O(Len).
func (m *KruskalMST) Edges() []dijkstra.Edge {
	if m == nil {
		return nil
	}
	return slices.Clone(m.edges)
}

// Weight returns the total weight of the tree (or forest).  A nil
// receiver reports 0.
// Complexity is O(1).
func (m *KruskalMST) Weight() float64 {
	if m == nil {
		return 0
	}
	return m.weight
}

// Len returns the number of tree edges: V-1 on a connected graph, fewer
// for a spanning forest.  A nil receiver reports 0.
// Complexity is O(1).
func (m *KruskalMST) Len() int {
	if m == nil {
		return 0
	}
	return len(m.edges)
}
