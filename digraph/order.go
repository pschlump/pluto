/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

// DepthFirstOrder computes the preorder, postorder, and reverse postorder
// of a digraph (Sedgwick's DepthFirstOrder, §4.5).  It is an immutable
// snapshot of the digraph at construction time: later AddEdge calls on
// the digraph are not reflected.
type DepthFirstOrder struct {
	pre         []int // vertices in preorder
	post        []int // vertices in postorder
	reversePost []int // vertices in reverse postorder
}

// NewDepthFirstOrder runs depth-first search over every vertex of g and
// records the three vertex orders.  A nil or empty digraph has empty
// orders — there is a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewDepthFirstOrder(g *Digraph) *DepthFirstOrder {
	o := &DepthFirstOrder{}
	if g == nil || g.n == 0 {
		return o
	}
	o.dfsAll(g.adj)
	return o
}

// dfsAll runs the depth-first search over adjacency lists, visiting every
// unmarked vertex in index order.
func (o *DepthFirstOrder) dfsAll(adj [][]int) {
	marked := make([]bool, len(adj))
	for v := 0; v < len(adj); v++ {
		if !marked[v] {
			o.dfs(adj, v, marked)
		}
	}
	// Reverse postorder is the postorder backwards.
	o.reversePost = make([]int, len(o.post))
	for i, v := range o.post {
		o.reversePost[len(o.post)-1-i] = v
	}
}

// dfs is the recursive depth-first search recording pre- and postorder.
func (o *DepthFirstOrder) dfs(adj [][]int, v int, marked []bool) {
	marked[v] = true
	o.pre = append(o.pre, v)
	for _, w := range adj[v] {
		if !marked[w] {
			o.dfs(adj, w, marked)
		}
	}
	o.post = append(o.post, v)
}

// Pre returns the vertices in preorder (the order DFS first visits them),
// as a fresh slice that the caller may mutate; nil for a nil receiver or
// an empty digraph.
// Complexity is O(V).
func (o *DepthFirstOrder) Pre() []int {
	if o == nil {
		return nil
	}
	return append([]int(nil), o.pre...)
}

// Post returns the vertices in postorder (the order DFS finishes them),
// as a fresh slice that the caller may mutate; nil for a nil receiver or
// an empty digraph.
// Complexity is O(V).
func (o *DepthFirstOrder) Post() []int {
	if o == nil {
		return nil
	}
	return append([]int(nil), o.post...)
}

// ReversePost returns the vertices in reverse postorder — a topological
// order when the digraph is acyclic — as a fresh slice that the caller
// may mutate; nil for a nil receiver or an empty digraph.
// Complexity is O(V).
func (o *DepthFirstOrder) ReversePost() []int {
	if o == nil {
		return nil
	}
	return append([]int(nil), o.reversePost...)
}
