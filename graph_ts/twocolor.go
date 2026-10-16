/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph_ts

// TwoColor answers bipartiteness queries on a graph (Sedgwick's
// TwoColor / Bipartite, §4.1): whether the vertices can be split into
// two sets such that every edge crosses between them — equivalently,
// whether the graph has no odd-length cycle.  It is an immutable
// snapshot of the graph at construction time: later AddEdge calls on
// the graph are not reflected, and a TwoColor is safe for concurrent
// reads.
type TwoColor struct {
	marked      []bool
	color       []bool
	isBipartite bool
}

// NewTwoColor snapshots the graph's adjacency lists under the read
// lock, then two-colors the snapshot lock-free.  A nil or empty graph
// is vacuously bipartite (IsBipartite is true and every Color query is
// out of range) — there is a sane answer, so nothing panics.  A
// self-loop makes the graph non-bipartite (it is an odd-length cycle);
// parallel edges never do (they form a 2-edge cycle).
// Complexity is O(V+E) time, O(V) space.
func NewTwoColor(g *Graph) *TwoColor {
	tc := &TwoColor{isBipartite: true}
	if g == nil {
		return tc
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		return tc
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	tc.marked = make([]bool, len(adj))
	tc.color = make([]bool, len(adj))
	for v := 0; v < len(adj); v++ {
		if !tc.marked[v] {
			tc.dfs(adj, v)
		}
	}
	return tc
}

// dfs is the recursive depth-first search: every tree edge flips the
// color, and any edge joining two same-colored vertices witnesses an
// odd cycle.  The search always runs to completion — unlike Cycle there
// is no short-circuit, because the coloring of the rest of the graph is
// still computed.  No lock is held while it runs.
func (tc *TwoColor) dfs(adj [][]int, v int) {
	tc.marked[v] = true
	for _, w := range adj[v] {
		if !tc.marked[w] {
			tc.color[w] = !tc.color[v]
			tc.dfs(adj, w)
		} else if tc.color[w] == tc.color[v] {
			tc.isBipartite = false
		}
	}
}

// IsBipartite reports whether the graph is bipartite (two-colorable).
// Complexity is O(1).
func (tc *TwoColor) IsBipartite() bool {
	if tc == nil {
		return false
	}
	return tc.isBipartite
}

// Color returns which side of the bipartition v is on.  The two sides
// are the two bool values; which one is which is an implementation
// detail (swap them for the other valid coloring).  ok is false if v is
// out of range (or the receiver is nil).  The colors are meaningful
// only when IsBipartite is true — on a non-bipartite graph the search
// stops refining them at the first odd cycle found.
// Complexity is O(1).
func (tc *TwoColor) Color(v int) (color bool, ok bool) {
	if tc == nil || v < 0 || v >= len(tc.color) {
		return false, false
	}
	return tc.color[v], true
}
