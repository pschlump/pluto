/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package mst

import (
	"slices"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/priority_queue"
)

// LazyPrimMST is the result of running the lazy version of Prim's
// algorithm (Sedgwick's LazyPrimMST) on an edge-weighted graph.  It is
// an immutable snapshot of the graph at construction time: later
// AddEdge calls on the graph are not reflected.
type LazyPrimMST struct {
	weight float64
	edges  []dijkstra.Edge
}

// NewLazyPrimMST computes a minimum spanning tree of g with the lazy
// version of Prim's algorithm: grow a tree from a start vertex, keep
// every crossing edge on a priority queue (pluto/priority_queue) keyed
// by weight, repeatedly add the lightest crossing edge, and lazily
// discard the stale entries whose endpoints are both already on the
// tree.
//
// On a disconnected graph the result is the minimum spanning FOREST:
// the tree is restarted from each unvisited vertex, so every connected
// component gets its own minimum tree and Edges holds V - (number of
// components) edges.
//
// It panics on a nil or empty graph — no spanning tree exists to
// compute.
// Complexity is O(E log E) time, O(E) space (the lazy queue can hold
// one entry per edge).
func NewLazyPrimMST(g *dijkstra.EdgeWeightedGraph) *LazyPrimMST {
	checkGraph("NewLazyPrimMST", g)
	mst := &LazyPrimMST{}
	n := g.V()
	marked := make([]bool, n)
	pq := priority_queue.NewPriorityQueueFunc(compareEdgeByWeight)
	for s := 0; s < n; s++ {
		if !marked[s] {
			lazyPrimFrom(g, mst, marked, pq, s)
		}
	}
	return mst
}

// lazyPrimFrom grows one tree of the forest from vertex s.
func lazyPrimFrom(g *dijkstra.EdgeWeightedGraph, mst *LazyPrimMST, marked []bool, pq *priority_queue.PriorityQueue[dijkstra.Edge], s int) {
	lazyScan(g, marked, pq, s)
	for !pq.IsEmpty() {
		e, _ := pq.Pop() // the lightest crossing edge known so far
		v, w := e.V, e.W
		if marked[v] && marked[w] {
			continue // stale: both endpoints are already on the tree
		}
		mst.edges = append(mst.edges, e)
		mst.weight += e.Weight
		if !marked[v] {
			lazyScan(g, marked, pq, v)
		}
		if !marked[w] {
			lazyScan(g, marked, pq, w)
		}
	}
}

// lazyScan marks v and pushes every edge from v to a not-yet-marked
// vertex onto the priority queue.
func lazyScan(g *dijkstra.EdgeWeightedGraph, marked []bool, pq *priority_queue.PriorityQueue[dijkstra.Edge], v int) {
	marked[v] = true
	for _, e := range g.Adj(v) {
		if !marked[e.Other(v)] {
			pq.Insert(e)
		}
	}
}

// Edges returns the edges of the minimum spanning tree (or forest, on a
// disconnected graph) as a fresh slice the caller may mutate.  A nil
// receiver reports nil.
// Complexity is O(Len).
func (m *LazyPrimMST) Edges() []dijkstra.Edge {
	if m == nil {
		return nil
	}
	return slices.Clone(m.edges)
}

// Weight returns the total weight of the tree (or forest).  A nil
// receiver reports 0.
// Complexity is O(1).
func (m *LazyPrimMST) Weight() float64 {
	if m == nil {
		return 0
	}
	return m.weight
}

// Len returns the number of tree edges: V-1 on a connected graph, fewer
// for a spanning forest.  A nil receiver reports 0.
// Complexity is O(1).
func (m *LazyPrimMST) Len() int {
	if m == nil {
		return 0
	}
	return len(m.edges)
}
