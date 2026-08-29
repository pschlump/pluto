/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package maxflow

import (
	"fmt"
	"math"

	"github.com/pschlump/pluto/queue"
)

// -------------------------------------------------------------------------------------------------------
// FordFulkerson — maximum s-t flow and minimum s-t cut
// -------------------------------------------------------------------------------------------------------

// FordFulkerson answers max-flow and min-cut queries for a source s and
// a sink t in a flow network (Sedgwick's FordFulkerson with the
// Edmonds-Karp shortest-augmenting-path rule: every augmenting path is
// a BFS shortest path in the residual network, on pluto's own
// queue.Queue — an intra-pluto composition, like graph's BFSPaths).
//
// The flow is computed eagerly in the constructor on a private deep
// copy of the network's edges, so the caller's FlowNetwork is never
// mutated and the result object is an immutable snapshot of the network
// at construction time (the mst/dijkstra precedent): later AddEdge
// calls on the network are not reflected.
type FordFulkerson struct {
	value  float64
	marked []bool      // marked[v]: v is on the source side of the min cut
	edges  []*FlowEdge // the private edge copy, flows at their max-flow values
	s      int
	t      int
}

// snapshot returns a private, mutable deep copy of the network's edges
// as pointer adjacency lists plus a flat list of every edge: each edge
// becomes one *FlowEdge shared by both endpoints' lists (the network
// itself stores two value copies, one per endpoint), so the algorithm's
// flow updates never touch the caller's network.
// Complexity is O(V + E).
func (g *FlowNetwork) snapshot() (adj [][]*FlowEdge, edges []*FlowEdge) {
	adj = make([][]*FlowEdge, g.n)
	edges = make([]*FlowEdge, 0, g.e)
	for v := 0; v < g.n; v++ {
		selfLoops := 0
		for _, e := range g.adj[v] {
			if e.From != v {
				continue // the copy in the tail vertex's list creates the edge
			}
			if e.To == v { // a self-loop is stored twice in adj[v]: create it once
				selfLoops++
				if selfLoops%2 == 0 {
					continue
				}
			}
			p := &FlowEdge{From: e.From, To: e.To, Capacity: e.Capacity}
			adj[e.From] = append(adj[e.From], p)
			adj[e.To] = append(adj[e.To], p)
			edges = append(edges, p)
		}
	}
	return adj, edges
}

// NewFordFulkerson computes a maximum s-t flow and a minimum s-t cut in
// g.  It panics on a nil or empty network, on an out-of-range source or
// sink, or when s == t — programmer errors with no sane answer (algs4
// throws IllegalArgumentException for the same cases).
// Complexity is O(V·E²) time in the worst case (Edmonds-Karp: O(V·E)
// augmentations, each a BFS in O(E)), much faster in practice, and
// O(V + E) space for the private edge copy.
func NewFordFulkerson(g *FlowNetwork, s, t int) *FordFulkerson {
	if g == nil || g.n == 0 {
		panic("maxflow: NewFordFulkerson called on a nil or empty network")
	}
	if s < 0 || s >= g.n {
		panic(fmt.Sprintf("maxflow: NewFordFulkerson called with out-of-range source %d (network has %d vertices)", s, g.n))
	}
	if t < 0 || t >= g.n {
		panic(fmt.Sprintf("maxflow: NewFordFulkerson called with out-of-range sink %d (network has %d vertices)", t, g.n))
	}
	if s == t {
		panic(fmt.Sprintf("maxflow: NewFordFulkerson called with source == sink (%d)", s))
	}

	adj, edges := g.snapshot()
	ff := &FordFulkerson{marked: make([]bool, g.n), edges: edges, s: s, t: t}
	edgeTo := make([]*FlowEdge, g.n)

	// While there is an augmenting path, augment along it by the
	// bottleneck residual capacity.  The final (failed) BFS leaves
	// ff.marked holding exactly the source side of the min cut.
	for ff.hasAugmentingPath(adj, s, t, edgeTo) {
		bottle := math.Inf(1)
		for v := t; v != s; v = edgeTo[v].Other(v) {
			bottle = min(bottle, edgeTo[v].ResidualCapacityTo(v))
		}
		for v := t; v != s; v = edgeTo[v].Other(v) {
			edgeTo[v].AddResidualFlowTo(v, bottle)
		}
		ff.value += bottle
	}
	return ff
}

// hasAugmentingPath runs a BFS from s in the residual network (the
// Edmonds-Karp shortest augmenting path); on return ff.marked holds the
// vertices reachable from s in the residual network and edgeTo a
// parent-link representation of a shortest residual s -> t path, if
// one exists.
// Complexity is O(V + E).
func (ff *FordFulkerson) hasAugmentingPath(adj [][]*FlowEdge, s, t int, edgeTo []*FlowEdge) bool {
	for v := range ff.marked {
		ff.marked[v] = false
	}
	var q queue.Queue[int]
	q.Push(s)
	ff.marked[s] = true
	for !q.IsEmpty() && !ff.marked[t] {
		v, _ := q.Dequeue()
		for _, e := range adj[v] {
			w := e.Other(v)
			if e.ResidualCapacityTo(w) > 0 && !ff.marked[w] {
				edgeTo[w] = e
				ff.marked[w] = true
				q.Push(w)
			}
		}
	}
	return ff.marked[t]
}

// Value returns the value of the maximum s-t flow (0 on a nil
// receiver).
// Complexity is O(1).
func (ff *FordFulkerson) Value() float64 {
	if ff == nil {
		return 0
	}
	return ff.value
}

// InMinCut reports whether v is on the source side of the minimum s-t
// cut.  Out-of-range vertices (and a nil receiver) report false — the
// pluto nil-tolerance convention; algs4 throws on an out-of-range
// vertex.
// Complexity is O(1).
func (ff *FordFulkerson) InMinCut(v int) bool {
	if ff == nil || v < 0 || v >= len(ff.marked) {
		return false
	}
	return ff.marked[v]
}

// S returns the source vertex the query was computed for (-1 on a nil
// receiver).
// Complexity is O(1).
func (ff *FordFulkerson) S() int {
	if ff == nil {
		return -1
	}
	return ff.s
}

// T returns the sink vertex the query was computed for (-1 on a nil
// receiver).
// Complexity is O(1).
func (ff *FordFulkerson) T() int {
	if ff == nil {
		return -1
	}
	return ff.t
}

// Edges returns the network's edges with their computed max-flow
// values, as a fresh slice of copies the caller may mutate.  The order
// is the internal adjacency order (edges grouped by tail vertex), not
// insertion order.  A nil receiver reports nil.
// Complexity is O(E).
func (ff *FordFulkerson) Edges() []FlowEdge {
	if ff == nil {
		return nil
	}
	out := make([]FlowEdge, len(ff.edges))
	for i, p := range ff.edges {
		out[i] = *p
	}
	return out
}
