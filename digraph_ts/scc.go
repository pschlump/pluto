/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

// KosarajuSCC answers strong-component queries on a digraph (Sedgwick's
// KosarajuSharirSCC, §4.6): two vertices are strongly connected when each
// is reachable from the other.  The algorithm runs the depth-first order
// on the reverse digraph, then a DFS on the digraph itself in
// reverse-postorder.  It is an immutable snapshot of the digraph at
// construction time: later AddEdge calls on the digraph are not
// reflected, and a KosarajuSCC is safe for concurrent reads.
type KosarajuSCC struct {
	id    []int
	count int
}

// NewKosarajuSCC snapshots the digraph's adjacency lists under the read
// lock, then computes the strongly connected components lock-free on the
// snapshot (reversing the snapshot for the first pass — no second lock
// acquisition).  A nil or empty digraph has no components (Count is 0 and
// every query reports not-connected) — there is a sane answer, so nothing
// panics.
// Complexity is O(V+E) time, O(V) space.
func NewKosarajuSCC(g *Digraph) *KosarajuSCC {
	s := &KosarajuSCC{}
	if g == nil {
		return s
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		return s
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()

	// First pass: depth-first order of the reversed snapshot.
	radj := make([][]int, len(adj))
	for v, ws := range adj {
		for _, w := range ws {
			radj[w] = append(radj[w], v)
		}
	}
	var order DepthFirstOrder
	order.dfsAll(radj)

	// Second pass: DFS on the snapshot in reverse-postorder.
	s.id = make([]int, len(adj))
	marked := make([]bool, len(adj))
	for _, v := range order.ReversePost() {
		if !marked[v] {
			s.dfs(adj, v, marked)
			s.count++
		}
	}
	return s
}

// dfs marks every vertex in v's strong component with the current
// component id.
func (s *KosarajuSCC) dfs(adj [][]int, v int, marked []bool) {
	marked[v] = true
	s.id[v] = s.count
	for _, w := range adj[v] {
		if !marked[w] {
			s.dfs(adj, w, marked)
		}
	}
}

// StronglyConnected reports whether v and w are in the same strong
// component (each reachable from the other).  Out-of-range vertices (and
// a nil receiver) report false.
// Complexity is O(1).
func (s *KosarajuSCC) StronglyConnected(v, w int) bool {
	if s == nil || v < 0 || v >= len(s.id) || w < 0 || w >= len(s.id) {
		return false
	}
	return s.id[v] == s.id[w]
}

// ID returns the strong-component identifier of v (0..Count()-1); ok is
// false if v is out of range.
// Complexity is O(1).
func (s *KosarajuSCC) ID(v int) (id int, ok bool) {
	if s == nil || v < 0 || v >= len(s.id) {
		return 0, false
	}
	return s.id[v], true
}

// Count returns the number of strong components.
// Complexity is O(1).
func (s *KosarajuSCC) Count() int {
	if s == nil {
		return 0
	}
	return s.count
}
