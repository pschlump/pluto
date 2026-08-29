/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

// KosarajuSCC answers strong-component queries on a digraph (Sedgwick's
// KosarajuSharirSCC, §4.6): two vertices are strongly connected when each
// is reachable from the other.  The algorithm runs DepthFirstOrder on the
// reverse digraph, then a DFS on the digraph itself in reverse-postorder.
// It is an immutable snapshot of the digraph at construction time: later
// AddEdge calls on the digraph are not reflected.
type KosarajuSCC struct {
	id    []int
	count int
}

// NewKosarajuSCC computes the strongly connected components of g.  A nil
// or empty digraph has no components (Count is 0 and every query reports
// not-connected) — there is a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewKosarajuSCC(g *Digraph) *KosarajuSCC {
	s := &KosarajuSCC{}
	if g == nil || g.n == 0 {
		return s
	}
	order := NewDepthFirstOrder(g.Reverse())
	s.id = make([]int, g.n)
	marked := make([]bool, g.n)
	for _, v := range order.ReversePost() {
		if !marked[v] {
			s.dfs(g.adj, v, marked)
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
