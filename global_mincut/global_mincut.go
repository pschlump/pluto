/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package global_mincut finds a minimum cut of an edge-weighted
// undirected graph — the bipartition of the vertices with the least
// total weight of crossing edges (Sedgick & Wayne, Algorithms 4th ed.
// §6.4) — via the Stoer–Wagner algorithm.
//
// It is a constructor-style query object over pluto/dijkstra's
// EdgeWeightedGraph — the same container mst consumes (an intra-pluto
// composition): NewGlobalMincut runs the whole algorithm eagerly and
// the result is an immutable snapshot; later AddEdge calls on the graph
// are not reflected.
//
//	NewGlobalMincut(g) — Stoer–Wagner min cut.	O(V³) time, O(V²) space
//
// Stoer–Wagner needs no source pair (unlike the max-flow route of n−1
// Ford–Fulkerson runs) and no edge ordering: it runs V−1 merge phases,
// each a maximum-adjacency search whose last vertex yields a candidate
// cut.  Ties in the search break toward the smallest vertex id, so the
// reported cut is deterministic for a given graph.
//
// Cut weights must be finite and non-negative — the constructor panics
// naming the first offending edge (the Dijkstra precedent; NaN compares
// false against every bound, so it is rejected explicitly, as is ±Inf).
// Parallel edges sum (capacity semantics); self-loops never cross a
// cut and are ignored.
//
// Panic contract (each message names the constructor and the fix):
// NewGlobalMincut on a nil or empty graph (no cut exists to compute) or
// on a negative/NaN/infinite edge weight.  Every other operation
// tolerates nil and zero values: a nil result reports Weight 0, no cut
// and no membership; a single-vertex graph has no bipartition and
// reports the same.
//
// This implementation is NOT safe for concurrent use, but the result
// object is immutable once constructed and safe for concurrent reads;
// there is no _ts twin (the suffix_array / maxflow precedent).
package global_mincut

import (
	"fmt"
	"math"
	"slices"

	"github.com/pschlump/pluto/dijkstra"
)

// GlobalMincut is the minimum cut of an edge-weighted undirected graph:
// the total weight of the crossing edges, the vertices of one side, and
// membership queries against that side.  It is an immutable snapshot of
// the graph at construction time.
type GlobalMincut struct {
	weight float64
	inCut  []bool // true = vertex is on the reported cut side
}

// NewGlobalMincut computes a minimum cut of g by the Stoer–Wagner
// algorithm: V−1 merge phases, each a maximum-adjacency search (ties
// break toward the smallest vertex id — deterministic) whose final
// vertex t yields the phase cut "t alone vs the rest"; the minimum over
// phases is the global minimum cut.  A disconnected graph reports
// weight 0 with one component as the cut side.
// Complexity is O(V³) time, O(V²) space.
func NewGlobalMincut(g *dijkstra.EdgeWeightedGraph) *GlobalMincut {
	c := &GlobalMincut{}
	if g == nil || g.V() == 0 {
		panic("global_mincut: NewGlobalMincut called on a nil or empty graph — a min cut needs at least one edge-crossable bipartition")
	}
	n := g.V()

	// Build the symmetric weight matrix (parallel edges sum, self-loops
	// are skipped — they never cross a cut), validating weights as we
	// go.  Each undirected edge is seen from both endpoints' lists, so
	// mat[v][u] and mat[u][v] each accumulate once.
	mat := make([][]float64, n)
	for i := range mat {
		mat[i] = make([]float64, n)
	}
	for v := 0; v < n; v++ {
		for _, e := range g.Adj(v) {
			u := e.Other(v)
			if u == v {
				continue
			}
			if math.IsNaN(e.Weight) || math.IsInf(e.Weight, 0) {
				panic(fmt.Sprintf("global_mincut: NewGlobalMincut: edge %d--%d has non-finite weight %g — cut weights must be finite and non-negative", e.V, e.W, e.Weight))
			}
			if e.Weight < 0 {
				panic(fmt.Sprintf("global_mincut: NewGlobalMincut: edge %d--%d has negative weight %g — cut weights must be finite and non-negative", e.V, e.W, e.Weight))
			}
			mat[v][u] += e.Weight
		}
	}

	// Super-vertex state: groups[v] holds the original vertices merged
	// into v; active[v] marks unmerged super-vertices.
	groups := make([][]int, n)
	for v := range groups {
		groups[v] = []int{v}
	}
	active := make([]bool, n)
	for v := range active {
		active[v] = true
	}

	best := math.Inf(1)
	var bestGroup []int
	conn := make([]float64, n) // connection of each vertex to the current merged set A
	added := make([]bool, n)

	for phase := 1; phase < n; phase++ {
		live := n - phase + 1 // active super-vertices at this phase (>= 2)
		for v := range conn {
			conn[v] = 0
			added[v] = false
		}

		// Maximum-adjacency search: repeatedly add the active vertex
		// most tightly connected to A (smallest id on ties).
		s, t := 0, 0
		for step := 0; step < live; step++ {
			pick := -1
			for v := 0; v < n; v++ {
				if active[v] && !added[v] && (pick == -1 || conn[v] > conn[pick]) {
					pick = v
				}
			}
			s, t = t, pick // second-to-last, last
			added[pick] = true
			for u := 0; u < n; u++ {
				if active[u] && !added[u] {
					conn[u] += mat[pick][u]
				}
			}
		}

		// The phase cut isolates t; conn[t] is its weight.  (For the
		// last phase live == 2, this is the final pair's cut.)
		if conn[t] < best {
			best = conn[t]
			bestGroup = append([]int(nil), groups[t]...)
		}

		// Merge t into s and deactivate t.
		for u := 0; u < n; u++ {
			if active[u] && u != s && u != t {
				mat[s][u] += mat[t][u]
				mat[u][s] = mat[s][u]
			}
		}
		groups[s] = append(groups[s], groups[t]...)
		active[t] = false
	}

	if bestGroup == nil {
		return c // n == 1: no bipartition exists — the zero result (weight 0, no cut)
	}
	c.weight = best
	c.inCut = make([]bool, n)
	for _, v := range bestGroup {
		c.inCut[v] = true
	}
	return c
}

// Weight returns the total weight of the minimum cut's crossing edges.
// It is 0 for a disconnected graph, a single-vertex graph and a nil
// result.
// Complexity is O(1).
func (c *GlobalMincut) Weight() float64 {
	if c == nil {
		return 0
	}
	return c.weight
}

// Cut returns the vertices of one side of a minimum cut, sorted, as a
// fresh slice that the caller may mutate.  It returns nil when no cut
// exists (a single-vertex graph, or a nil result) — for every graph
// with at least two vertices some cut exists, weight 0 included.
// Complexity is O(V).
func (c *GlobalMincut) Cut() []int {
	if c == nil || c.inCut == nil {
		return nil
	}
	cut := make([]int, 0, len(c.inCut))
	for v, in := range c.inCut {
		if in {
			cut = append(cut, v)
		}
	}
	slices.Sort(cut)
	return cut
}

// InCut reports whether v is on the side returned by Cut.  Out-of-range
// vertices (and a nil result, or a graph with no cut) report false.
// Complexity is O(1).
func (c *GlobalMincut) InCut(v int) bool {
	if c == nil || v < 0 || v >= len(c.inCut) {
		return false
	}
	return c.inCut[v]
}
