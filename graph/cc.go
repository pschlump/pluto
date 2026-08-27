/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph

// CC answers connected-component queries on a graph (Sedgwick's CC).  It
// is an immutable snapshot of the graph at construction time: later
// AddEdge calls on the graph are not reflected.
type CC struct {
	id    []int
	count int
}

// NewCC computes the connected components of g.  A nil or empty graph has
// no components (Count is 0 and every query reports not-connected) —
// unlike the Paths constructors there is a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewCC(g *Graph) *CC {
	c := &CC{}
	if g == nil || g.n == 0 {
		return c
	}
	c.id = make([]int, g.n)
	marked := make([]bool, g.n)
	for v := 0; v < g.n; v++ {
		if !marked[v] {
			c.dfs(g.adj, v, marked)
			c.count++
		}
	}
	return c
}

// dfs marks every vertex in v's component with the current component id.
func (c *CC) dfs(adj [][]int, v int, marked []bool) {
	marked[v] = true
	c.id[v] = c.count
	for _, w := range adj[v] {
		if !marked[w] {
			c.dfs(adj, w, marked)
		}
	}
}

// Connected reports whether v and w are in the same component.
// Out-of-range vertices (and a nil receiver) report false.
// Complexity is O(1).
func (c *CC) Connected(v, w int) bool {
	if c == nil || v < 0 || v >= len(c.id) || w < 0 || w >= len(c.id) {
		return false
	}
	return c.id[v] == c.id[w]
}

// ID returns the component identifier of v (0..Count()-1); ok is false if
// v is out of range.
// Complexity is O(1).
func (c *CC) ID(v int) (id int, ok bool) {
	if c == nil || v < 0 || v >= len(c.id) {
		return 0, false
	}
	return c.id[v], true
}

// Count returns the number of connected components.
// Complexity is O(1).
func (c *CC) Count() int {
	if c == nil {
		return 0
	}
	return c.count
}
