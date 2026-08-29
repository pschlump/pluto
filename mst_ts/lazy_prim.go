/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package mst_ts

import (
	"slices"

	"github.com/pschlump/pluto/dijkstra_ts"
	"github.com/pschlump/pluto/priority_queue"
)

// LazyPrimMST is the result of running the lazy version of Prim's
// algorithm (Sedgwick's LazyPrimMST) on an edge-weighted graph.  It is
// an immutable snapshot of the graph at construction time — later
// AddEdge calls are not reflected — and is safe for concurrent reads.
type LazyPrimMST struct {
	weight float64
	edges  []dijkstra_ts.Edge
}

// NewLazyPrimMST snapshots the graph's adjacency lists (each under the
// graph's read lock), then computes a minimum spanning tree lock-free
// on the snapshot with the lazy version of Prim's algorithm: grow a
// tree from a start vertex, keep every crossing edge on a priority
// queue (pluto/priority_queue) keyed by weight, repeatedly add the
// lightest crossing edge, and lazily discard the stale entries whose
// endpoints are both already on the tree.
//
// On a disconnected graph the result is the minimum spanning FOREST:
// the tree is restarted from each unvisited vertex, so every connected
// component gets its own minimum tree and Edges holds V - (number of
// components) edges.
//
// It panics on a nil or empty graph — no spanning tree exists to
// compute.
// Complexity is O(E log E) time plus O(V+E) for the snapshot, O(E)
// space.
func NewLazyPrimMST(g *dijkstra_ts.EdgeWeightedGraph) *LazyPrimMST {
	checkGraph("NewLazyPrimMST", g)
	return lazyPrimFrom(snapshotAdj(g))
}

// lazyPrimFrom is the lock-free core: lazy Prim over the snapshot
// adjacency adj, restarted from every unvisited vertex so a
// disconnected graph yields the spanning forest.
func lazyPrimFrom(adj [][]dijkstra_ts.Edge) *LazyPrimMST {
	mst := &LazyPrimMST{}
	marked := make([]bool, len(adj))
	pq := priority_queue.NewPriorityQueueFunc(compareEdgeByWeight)
	for s := range adj {
		if !marked[s] {
			lazyPrimTree(adj, mst, marked, pq, s)
		}
	}
	return mst
}

// lazyPrimTree grows one tree of the forest from vertex s.
func lazyPrimTree(adj [][]dijkstra_ts.Edge, mst *LazyPrimMST, marked []bool, pq *priority_queue.PriorityQueue[dijkstra_ts.Edge], s int) {
	lazyScan(adj, marked, pq, s)
	for !pq.IsEmpty() {
		e, _ := pq.Pop() // the lightest crossing edge known so far
		v, w := e.V, e.W
		if marked[v] && marked[w] {
			continue // stale: both endpoints are already on the tree
		}
		mst.edges = append(mst.edges, e)
		mst.weight += e.Weight
		if !marked[v] {
			lazyScan(adj, marked, pq, v)
		}
		if !marked[w] {
			lazyScan(adj, marked, pq, w)
		}
	}
}

// lazyScan marks v and pushes every edge from v to a not-yet-marked
// vertex onto the priority queue.
func lazyScan(adj [][]dijkstra_ts.Edge, marked []bool, pq *priority_queue.PriorityQueue[dijkstra_ts.Edge], v int) {
	marked[v] = true
	for _, e := range adj[v] {
		if !marked[e.Other(v)] {
			pq.Insert(e)
		}
	}
}

// Edges returns the edges of the minimum spanning tree (or forest, on a
// disconnected graph) as a fresh slice the caller may mutate.  A nil
// receiver reports nil.
// Complexity is O(Len).
func (m *LazyPrimMST) Edges() []dijkstra_ts.Edge {
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
