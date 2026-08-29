/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

// DirectedCycle finds a directed cycle in a digraph (Sedgwick's
// DirectedCycle, §4.4) using on-stack DFS marking.  It is an immutable
// snapshot of the digraph at construction time: later AddEdge calls on
// the digraph are not reflected.
type DirectedCycle struct {
	marked  []bool
	onStack []bool
	edgeTo  []int
	cycle   []int // one directed cycle, in edge order, first vertex repeated at the end
}

// NewDirectedCycle searches g for a directed cycle.  A nil or empty
// digraph has no cycle (HasCycle is false and Cycle is nil) — there is a
// sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewDirectedCycle(g *Digraph) *DirectedCycle {
	c := &DirectedCycle{}
	if g == nil || g.n == 0 {
		return c
	}
	c.find(g.adj)
	return c
}

// find runs the cycle search over adjacency lists.
func (c *DirectedCycle) find(adj [][]int) {
	c.marked = make([]bool, len(adj))
	c.onStack = make([]bool, len(adj))
	c.edgeTo = make([]int, len(adj))
	for v := 0; v < len(adj); v++ {
		if !c.marked[v] {
			c.dfs(adj, v)
		}
	}
}

// dfs is the recursive depth-first search; an edge to an on-stack vertex
// closes a cycle.
func (c *DirectedCycle) dfs(adj [][]int, v int) {
	c.onStack[v] = true
	c.marked[v] = true
	for _, w := range adj[v] {
		if len(c.cycle) > 0 {
			return // short-circuit once a cycle is found
		}
		if !c.marked[w] {
			c.edgeTo[w] = v
			c.dfs(adj, w)
		} else if c.onStack[w] {
			// Cycle w -> ... -> v -> w, recorded in edge order with w
			// repeated at the end.
			var rev []int
			for x := v; x != w; x = c.edgeTo[x] {
				rev = append(rev, x)
			}
			c.cycle = append(c.cycle, w)
			for i := len(rev) - 1; i >= 0; i-- {
				c.cycle = append(c.cycle, rev[i])
			}
			c.cycle = append(c.cycle, w)
			return
		}
	}
	c.onStack[v] = false
}

// HasCycle reports whether the digraph has a directed cycle.
// Complexity is O(1).
func (c *DirectedCycle) HasCycle() bool {
	if c == nil {
		return false
	}
	return len(c.cycle) > 0
}

// Cycle returns a directed cycle as a list of vertices in edge order with
// the first vertex repeated at the end (a self-loop at v is [v v]), as a
// fresh slice that the caller may mutate.  It returns nil when the
// digraph has no cycle (or on a nil receiver).
// Complexity is O(cycle length).
func (c *DirectedCycle) Cycle() []int {
	if !c.HasCycle() {
		return nil
	}
	return append([]int(nil), c.cycle...)
}
