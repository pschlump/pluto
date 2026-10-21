/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

// GabowSCC answers strong-component queries on a digraph (Sedgwick's
// GabowSCC, §4.2): two vertices are strongly connected when each is
// reachable from the other.  The algorithm is Gabow's path-based
// single-pass DFS — like TarjanSCC it needs no reverse graph, but it
// replaces the low-link bookkeeping with two vertex stacks: the path
// stack (every vertex on the current DFS path) and the boundary stack
// (the subset that are still candidate component roots).  It is an
// immutable snapshot of the digraph at construction time: later AddEdge
// calls on the digraph are not reflected, and a GabowSCC is safe for
// concurrent reads.
type GabowSCC struct {
	marked   []bool
	id       []int
	preorder []int
	pre      int
	count    int
	stack1   []int // path stack: every vertex on the current DFS path
	stack2   []int // boundary stack: still-candidate component roots
}

// NewGabowSCC snapshots the digraph's adjacency lists under the read
// lock, then computes the strongly connected components lock-free on the
// snapshot (one pass, no second lock acquisition).  A nil or empty
// digraph has no components (Count is 0 and every query reports
// not-connected) — there is a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewGabowSCC(g *Digraph) *GabowSCC {
	s := &GabowSCC{}
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
	s.id = make([]int, len(adj)) // -1 until the vertex's component closes
	for v := range s.id {
		s.id[v] = -1
	}
	s.preorder = make([]int, len(adj))
	for v := 0; v < len(adj); v++ {
		if !s.marked[v] {
			s.dfs(adj, v)
		}
	}
	return s
}

// dfs is Gabow's recursive pass.  An edge to a visited-but-unclosed w
// (still on the path, id[w] == -1) retires every boundary discovered
// after w — they cannot be roots.  When control returns to v and v is
// still a boundary, v is a component root and the path stack down to v
// is its component.
func (s *GabowSCC) dfs(adj [][]int, v int) {
	s.marked[v] = true
	min := s.pre
	s.pre++
	s.preorder[v] = min
	s.stack1 = append(s.stack1, v)
	s.stack2 = append(s.stack2, v)
	for _, w := range adj[v] {
		if !s.marked[w] {
			s.dfs(adj, w)
		} else if s.id[w] == -1 {
			// w is on the current path: boundaries discovered after w
			// are not component roots.
			for s.preorder[s.stack2[len(s.stack2)-1]] > s.preorder[w] {
				s.stack2 = s.stack2[:len(s.stack2)-1]
			}
		}
	}
	if s.stack2[len(s.stack2)-1] != v {
		return // v was retired as a boundary: not a component root
	}
	s.stack2 = s.stack2[:len(s.stack2)-1]
	var w int
	for {
		w, s.stack1 = s.stack1[len(s.stack1)-1], s.stack1[:len(s.stack1)-1]
		s.id[w] = s.count
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
func (s *GabowSCC) StronglyConnected(v, w int) bool {
	if s == nil || v < 0 || v >= len(s.id) || w < 0 || w >= len(s.id) {
		return false
	}
	return s.id[v] == s.id[w]
}

// ID returns the strong-component identifier of v (0..Count()-1); ok is
// false if v is out of range.
// Complexity is O(1).
func (s *GabowSCC) ID(v int) (id int, ok bool) {
	if s == nil || v < 0 || v >= len(s.id) {
		return 0, false
	}
	return s.id[v], true
}

// Count returns the number of strong components.
// Complexity is O(1).
func (s *GabowSCC) Count() int {
	if s == nil {
		return 0
	}
	return s.count
}
