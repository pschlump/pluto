/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package mst_ts

import (
	"math"
	"slices"

	"github.com/pschlump/pluto/dijkstra_ts"
	"github.com/pschlump/pluto/index_pq"
)

// PrimMST is the result of running the eager version of Prim's
// algorithm (Sedgwick's PrimMST) on an edge-weighted graph.  It is an
// immutable snapshot of the graph at construction time — later AddEdge
// calls are not reflected — and is safe for concurrent reads.
type PrimMST struct {
	weight float64
	edges  []dijkstra_ts.Edge
}

// NewPrimMST snapshots the graph's adjacency lists (each under the
// graph's read lock), then computes a minimum spanning tree lock-free
// on the snapshot with the eager version of Prim's algorithm: grow a
// tree from a start vertex, keeping for each non-tree vertex only the
// LIGHTEST edge connecting it to the tree (distTo/edgeTo) in an indexed
// priority queue (pluto/index_pq), with Change as the decrease-key
// operation.
//
// On a disconnected graph the result is the minimum spanning FOREST:
// the tree is restarted from each unvisited vertex, so every connected
// component gets its own minimum tree and Edges holds V - (number of
// components) edges.
//
// It panics on a nil or empty graph — no spanning tree exists to
// compute.
// Complexity is O(E log V) time plus O(V+E) for the snapshot, O(V)
// space.
func NewPrimMST(g *dijkstra_ts.EdgeWeightedGraph) *PrimMST {
	checkGraph("NewPrimMST", g)
	return primFrom(snapshotAdj(g))
}

// primFrom is the lock-free core: eager Prim over the snapshot
// adjacency adj, restarted from every unvisited vertex so a
// disconnected graph yields the spanning forest.
func primFrom(adj [][]dijkstra_ts.Edge) *PrimMST {
	n := len(adj)
	distTo := make([]float64, n)          // distTo[v] is the weight of the lightest known edge from v to the tree
	edgeTo := make([]dijkstra_ts.Edge, n) // edgeTo[v] is that edge
	hasEdge := make([]bool, n)            // hasEdge[v] marks whether edgeTo[v] holds a real edge
	marked := make([]bool, n)             // marked[v] once v is on the tree
	for v := range distTo {
		distTo[v] = math.Inf(1)
	}
	for s := 0; s < n; s++ {
		if !marked[s] {
			primTree(adj, distTo, edgeTo, hasEdge, marked, s)
		}
	}
	mst := &PrimMST{}
	for v := 0; v < n; v++ {
		if hasEdge[v] {
			mst.edges = append(mst.edges, edgeTo[v])
			mst.weight += edgeTo[v].Weight
		}
	}
	return mst
}

// primTree grows one tree of the forest from vertex s.
func primTree(adj [][]dijkstra_ts.Edge, distTo []float64, edgeTo []dijkstra_ts.Edge, hasEdge, marked []bool, s int) {
	pq := index_pq.NewIndexPQ[float64](len(distTo))
	distTo[s] = 0
	pq.Insert(s, 0)
	for !pq.IsEmpty() {
		v, _, _ := pq.Pop() // add the closest non-tree vertex to the tree
		marked[v] = true
		for _, e := range adj[v] {
			w := e.Other(v)
			if marked[w] {
				continue // v--w is obsolete once both endpoints are on the tree
			}
			if e.Weight < distTo[w] { // a lighter edge from w to the tree
				distTo[w] = e.Weight
				edgeTo[w] = e
				hasEdge[w] = true
				if pq.Contains(w) {
					pq.Change(w, e.Weight) // decrease-key
				} else {
					pq.Insert(w, e.Weight)
				}
			}
		}
	}
}

// Edges returns the edges of the minimum spanning tree (or forest, on a
// disconnected graph) as a fresh slice the caller may mutate.  A nil
// receiver reports nil.
// Complexity is O(Len).
func (m *PrimMST) Edges() []dijkstra_ts.Edge {
	if m == nil {
		return nil
	}
	return slices.Clone(m.edges)
}

// Weight returns the total weight of the tree (or forest).  A nil
// receiver reports 0.
// Complexity is O(1).
func (m *PrimMST) Weight() float64 {
	if m == nil {
		return 0
	}
	return m.weight
}

// Len returns the number of tree edges: V-1 on a connected graph, fewer
// for a spanning forest.  A nil receiver reports 0.
// Complexity is O(1).
func (m *PrimMST) Len() int {
	if m == nil {
		return 0
	}
	return len(m.edges)
}
