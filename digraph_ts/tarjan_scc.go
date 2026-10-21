/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

// TarjanSCC answers strong-component queries on a digraph (Sedgwick's
// TarjanSCC, §4.2): two vertices are strongly connected when each is
// reachable from the other.  The algorithm is Tarjan's single-pass
// low-link DFS — one depth-first search over the digraph itself (no
// reverse graph, no second pass), popping each component off a vertex
// stack when the search retreats out of its root.  It is an immutable
// snapshot of the digraph at construction time: later AddEdge calls on
// the digraph are not reflected, and a TarjanSCC is safe for concurrent
// reads.
type TarjanSCC struct {
	marked []bool
	id     []int
	low    []int
	pre    int
	count  int
	stack  []int // vertices whose component is not yet closed
}

// NewTarjanSCC snapshots the digraph's adjacency lists under the read
// lock, then computes the strongly connected components lock-free on the
// snapshot (one pass, no second lock acquisition).  A nil or empty
// digraph has no components (Count is 0 and every query reports
// not-connected) — there is a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewTarjanSCC(g *Digraph) *TarjanSCC {
	s := &TarjanSCC{}
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

	s.marked = make([]bool, len(adj))
	s.id = make([]int, len(adj))
	s.low = make([]int, len(adj))
	for v := 0; v < len(adj); v++ {
		if !s.marked[v] {
			s.dfs(adj, v)
		}
	}
	return s
}

// dfs is Tarjan's recursive pass: low[v] is the smallest preorder number
// reachable from v's subtree using at most one edge back into the open
// stack (the algs4 form — a closed vertex gets low = V so it can never
// lower a later minimum).  When v's low equals its own preorder, v is
// the root of a component: everything above it on the stack (v included)
// is that component.
func (s *TarjanSCC) dfs(adj [][]int, v int) {
	s.marked[v] = true
	min := s.pre
	s.pre++
	s.low[v] = min
	s.stack = append(s.stack, v)
	for _, w := range adj[v] {
		if !s.marked[w] {
			s.dfs(adj, w)
		}
		if s.low[w] < min {
			min = s.low[w]
		}
	}
	if min < s.low[v] {
		s.low[v] = min
		return // v is not a component root
	}
	var w int
	for {
		w, s.stack = s.stack[len(s.stack)-1], s.stack[:len(s.stack)-1]
		s.id[w] = s.count
		s.low[w] = len(adj) // closed: no longer on the open stack
		if w == v {
			break
		}
	}
	s.count++
}

// StronglyConnected reports whether v and w are in the same strong
// component (each reachable from the other).  Out-of-range vertices (and
// a nil receiver) report false.
// Complexity is O(1).
func (s *TarjanSCC) StronglyConnected(v, w int) bool {
	if s == nil || v < 0 || v >= len(s.id) || w < 0 || w >= len(s.id) {
		return false
	}
	return s.id[v] == s.id[w]
}

// ID returns the strong-component identifier of v (0..Count()-1); ok is
// false if v is out of range.
// Complexity is O(1).
func (s *TarjanSCC) ID(v int) (id int, ok bool) {
	if s == nil || v < 0 || v >= len(s.id) {
		return 0, false
	}
	return s.id[v], true
}

// Count returns the number of strong components.
// Complexity is O(1).
func (s *TarjanSCC) Count() int {
	if s == nil {
		return 0
	}
	return s.count
}
