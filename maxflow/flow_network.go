/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package maxflow implements the Ford-Fulkerson max-flow / min-cut
// algorithm with the Edmonds-Karp shortest-augmenting-path rule
// (breadth-first search) over capacitated flow networks on the integer
// vertices 0..n-1 (Sedgwick, Algorithms, 4th ed., §6.4).  The vertices
// ARE the indices, like pluto/graph and pluto/dijkstra; there is no
// element type.
//
// Network type:
//
//	NewFlowNetwork(v) — a network on vertices 0..v-1.		O(v)
//	AddEdge(from, to, capacity) — add the edge from -> to.		O(1)
//	V() / E() — the number of vertices / edges.			O(1)
//	Adj(v) — the edges incident to v (both directions).		O(1)
//
// Query (an immutable snapshot of the network at construction time):
//
//	NewFordFulkerson(g, s, t) — max s-t flow and min s-t cut.	O(V·E²) worst case
//
// Parallel edges and self-loops are allowed, exactly like pluto/graph
// (a self-loop can never carry useful flow — it leaves and enters the
// same vertex).  There is deliberately no RemoveEdge — matching
// Sedgwick.
//
// Capacities and flows are float64 (the dijkstra weight precedent).
// Integer capacities stay exact under float64 summation as long as the
// maxflow value does not exceed 2⁵², so all arithmetic in that regime
// is exact.
//
// A nil *FlowNetwork and the zero value behave as an empty network for
// every read (V and E return 0, Adj returns nil).  A nil *FordFulkerson
// behaves as "no flow": Value returns 0, InMinCut false, S/T -1 and
// Edges nil.
//
// Panic contract (each panic message names the method and the fix):
// NewFlowNetwork with v < 1; AddEdge on a nil network or with a
// negative or NaN capacity; AddResidualFlowTo with a negative or NaN
// delta, or driving a flow outside [0, Capacity]; NewFordFulkerson on a
// nil or empty network, with an out-of-range source or sink, or with
// s == t.  Every other operation tolerates nil and zero values as an
// empty result.
//
// This implementation is NOT safe for concurrent use.  There is no
// maxflow_ts twin (the suffix_array documented-exception precedent):
// NewFordFulkerson computes on a private deep copy of the network's
// edges, so the result object is immutable and safe for concurrent
// reads once constructed.
package maxflow

import (
	"fmt"
)

// FlowEdge is a capacitated directed edge From -> To with a flow
// (Sedgwick's FlowEdge).  The fields are exported and read-only by
// convention (the dijkstra.Edge precedent): FordFulkerson maintains the
// flows on its own private copy of the edges, so the edges stored in a
// FlowNetwork always have Flow == 0 unless the caller mutates them
// through the slice returned by Adj — don't.
type FlowEdge struct {
	From, To       int
	Capacity, Flow float64
}

// Other returns the endpoint of e that is not v (for a self-loop it
// returns v itself).  v must be one of the endpoints.
// Complexity is O(1).
func (e FlowEdge) Other(v int) int {
	if v == e.From {
		return e.To
	}
	return e.From
}

// ResidualCapacityTo returns the residual capacity of e in the
// direction toward v: Capacity - Flow when v is the head (the forward
// direction), Flow when v is the tail (the backward direction — flow
// that can be canceled).  v must be one of the endpoints.
// Complexity is O(1).
func (e FlowEdge) ResidualCapacityTo(v int) float64 {
	if v == e.From {
		return e.Flow
	}
	return e.Capacity - e.Flow
}

// AddResidualFlowTo adds delta to the flow on e in the direction toward
// v: it increases Flow when v is the head and decreases Flow when v is
// the tail.  v must be one of the endpoints.  It panics, leaving the
// edge unchanged, if delta is negative or NaN, or if the update would
// drive the flow below 0 or above Capacity — programmer errors with no
// sane answer.  Unlike the
// algs4 Java there is no epsilon rounding: with integer capacities all
// arithmetic is exact.
// Complexity is O(1).
func (e *FlowEdge) AddResidualFlowTo(v int, delta float64) {
	if !(delta >= 0) {
		panic(fmt.Sprintf("maxflow: AddResidualFlowTo called with negative or NaN delta %g", delta))
	}
	flow := e.Flow
	if v == e.From {
		flow -= delta
	} else {
		flow += delta
	}
	if flow < 0 || flow > e.Capacity {
		panic(fmt.Sprintf("maxflow: AddResidualFlowTo on edge %d->%d drove the flow to %g (capacity %g)", e.From, e.To, flow, e.Capacity))
	}
	e.Flow = flow
}

// -------------------------------------------------------------------------------------------------------
// FlowNetwork
// -------------------------------------------------------------------------------------------------------

// FlowNetwork is a capacitated flow network on the vertices 0..n-1,
// stored as adjacency lists (Sedgwick's FlowNetwork).  Like algs4 — and
// like dijkstra's undirected EdgeWeightedGraph — each edge is recorded
// in BOTH endpoints' adjacency lists, because residual-capacity
// traversal needs the edges entering a vertex as well as the edges
// leaving it.
type FlowNetwork struct {
	n   int
	e   int
	adj [][]FlowEdge
}

// NewFlowNetwork creates a flow network with v vertices (0..v-1) and
// no edges.
// It panics if v < 1.
// Complexity is O(v).
func NewFlowNetwork(v int) *FlowNetwork {
	if v < 1 {
		panic(fmt.Sprintf("maxflow: NewFlowNetwork called with v=%d, need v >= 1", v))
	}
	return &FlowNetwork{n: v, adj: make([][]FlowEdge, v)}
}

// AddEdge adds the directed edge from -> to with the given capacity and
// zero initial flow, and returns true: the edge is recorded in both
// endpoints' adjacency lists, so residual traversal sees it in both
// directions.  It returns false if either endpoint is out of range (on
// a zero-value network every endpoint is out of range, so AddEdge
// reports false — the dijkstra precedent).  Parallel edges and
// self-loops are allowed; each call counts once in E, and a self-loop
// appears twice in Adj(from) — the dijkstra undirected self-loop
// convention.
// It panics on a nil network — a nil network cannot store an edge — and
// on a negative or NaN capacity (no sane answer; algs4 throws too).
// Complexity is O(1).
func (g *FlowNetwork) AddEdge(from, to int, capacity float64) bool {
	if g == nil {
		panic("maxflow: AddEdge called on a nil network")
	}
	if !(capacity >= 0) {
		panic(fmt.Sprintf("maxflow: AddEdge called with negative or NaN capacity %g", capacity))
	}
	if from < 0 || from >= g.n || to < 0 || to >= g.n {
		return false
	}
	e := FlowEdge{From: from, To: to, Capacity: capacity}
	g.adj[from] = append(g.adj[from], e)
	g.adj[to] = append(g.adj[to], e)
	g.e++
	return true
}

// V returns the number of vertices in the network.
// Complexity is O(1).
func (g *FlowNetwork) V() int {
	if g == nil {
		return 0
	}
	return g.n
}

// E returns the number of edges in the network (a self-loop counts
// once).
// Complexity is O(1).
func (g *FlowNetwork) E() int {
	if g == nil {
		return 0
	}
	return g.e
}

// Adj returns the edges incident to v — both the edges leaving v and
// the edges entering v — in insertion order, or nil if v is out of
// range or the network is nil.  The returned slice aliases the
// network's internal storage: it reflects later AddEdge calls and must
// not be mutated by the caller (each edge is stored by value in both
// endpoints' lists, so mutating one copy desynchronizes the pair).
// Complexity is O(1).
func (g *FlowNetwork) Adj(v int) []FlowEdge {
	if g == nil || v < 0 || v >= g.n {
		return nil
	}
	return g.adj[v]
}
