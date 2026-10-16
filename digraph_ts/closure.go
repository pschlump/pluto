/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

// TransitiveClosure answers all-pairs reachability queries on a digraph
// (Sedgwick's TransitiveClosure, §4.4): is w reachable from v by some
// directed path?  It is an immutable snapshot of the digraph at
// construction time: later AddEdge calls on the digraph are not
// reflected, and a TransitiveClosure is safe for concurrent reads.
//
// It is built the algs4 way — one depth-first search per vertex, the
// V DirectedDFS passes collapsed into a V×V boolean matrix — which for
// the sparse adjacency-list digraphs this package stores costs O(V·(V+E))
// time, far below Warshall's O(V³) dense closure.  The price is the
// O(V²) memory for the matrix: fine for hundreds of vertices, deliberate
// for thousands, prohibitive past a few tens of thousands.
type TransitiveClosure struct {
	reach [][]bool // reach[v][w]: w is reachable from v (v reaches itself)
}

// NewTransitiveClosure snapshots the digraph's adjacency lists under
// the read lock, then computes the reachability matrix lock-free on the
// snapshot.  A nil or empty digraph has no reachable pairs (every
// Reachable query reports false) — there is a sane answer, so nothing
// panics.  Every vertex reaches itself, even with no edges.
// Complexity is O(V·(V+E)) time, O(V²) space.
func NewTransitiveClosure(g *Digraph) *TransitiveClosure {
	tc := &TransitiveClosure{}
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
	n := len(adj)
	tc.reach = make([][]bool, n)
	marked := make([]bool, n) // scratch, cleared between the V passes
	for v := 0; v < n; v++ {
		clear(marked)
		r := make([]bool, n)
		tc.dfs(adj, marked, r, v)
		tc.reach[v] = r
	}
	return tc
}

// dfs is the recursive depth-first search from v, recording every
// reached vertex in r.  No lock is held while it runs.
func (tc *TransitiveClosure) dfs(adj [][]int, marked, r []bool, v int) {
	marked[v] = true
	r[v] = true
	for _, w := range adj[v] {
		if !marked[w] {
			tc.dfs(adj, marked, r, w)
		}
	}
}

// Reachable reports whether w is reachable from v by a directed path (a
// vertex always reaches itself; Reachable(v, v) is true even for an
// isolated v).  Out-of-range vertices (and a nil receiver) report
// false.
// Complexity is O(1).
func (tc *TransitiveClosure) Reachable(v, w int) bool {
	if tc == nil || v < 0 || v >= len(tc.reach) || w < 0 || w >= len(tc.reach) {
		return false
	}
	return tc.reach[v][w]
}
