/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph_ts

// Cycle finds a cycle in an undirected graph (Sedgwick's Cycle, §4.1).
// It is an immutable snapshot of the graph at construction time: later
// AddEdge calls on the graph are not reflected, and a Cycle is safe for
// concurrent reads.
//
// Self-loops and parallel edges are cycles (a self-loop at v is the
// 1-edge cycle [v v]; two parallel edges v-w are the 2-edge cycle
// [v w v]) and are found by dedicated scans before the depth-first
// search — the DFS parent check alone cannot see them, because the
// second copy of an edge v-w is indistinguishable from the edge back to
// the parent.
type Cycle struct {
	marked []bool
	edgeTo []int
	cycle  []int // one cycle, in edge order, first vertex repeated at the end
}

// NewCycle snapshots the graph's adjacency lists under the read lock,
// then searches the snapshot lock-free for a cycle.  A nil or empty
// graph has no cycle (HasCycle is false and Cycle is nil) — there is a
// sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewCycle(g *Graph) *Cycle {
	c := &Cycle{}
	if g == nil {
		return c
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		return c
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	c.find(adj)
	return c
}

// find runs the cycle search over adjacency lists; no lock is held
// while it runs.
func (c *Cycle) find(adj [][]int) {
	if c.hasSelfLoop(adj) || c.hasParallelEdges(adj) {
		return // the special-case scans already recorded the cycle
	}
	c.marked = make([]bool, len(adj))
	c.edgeTo = make([]int, len(adj))
	for v := 0; v < len(adj); v++ {
		if !c.marked[v] {
			c.dfs(adj, -1, v)
		}
	}
}

// hasSelfLoop reports whether some vertex is adjacent to itself,
// recording the 1-edge cycle [v v] if so.
func (c *Cycle) hasSelfLoop(adj [][]int) bool {
	for v := 0; v < len(adj); v++ {
		for _, w := range adj[v] {
			if v == w {
				c.cycle = []int{v, v}
				return true
			}
		}
	}
	return false
}

// hasParallelEdges reports whether some two vertices are joined by more
// than one edge, recording the 2-edge cycle [v w v] if so.  It runs only
// after hasSelfLoop, so a marked w can never be v itself.
func (c *Cycle) hasParallelEdges(adj [][]int) bool {
	marked := make([]bool, len(adj))
	for v := 0; v < len(adj); v++ {
		for _, w := range adj[v] {
			if marked[w] {
				c.cycle = []int{v, w, v}
				return true
			}
			marked[w] = true
		}
		// Reset so marked stays a set of v's neighbors only.
		for _, w := range adj[v] {
			marked[w] = false
		}
	}
	return false
}

// dfs is the recursive depth-first search; an edge to a marked vertex
// that is not the parent u closes a cycle.
func (c *Cycle) dfs(adj [][]int, u, v int) {
	c.marked[v] = true
	for _, w := range adj[v] {
		if len(c.cycle) > 0 {
			return // short-circuit once a cycle is found
		}
		if !c.marked[w] {
			c.edgeTo[w] = v
			c.dfs(adj, v, w)
		} else if w != u {
			// Cycle w -> ... -> v -> w: the tree path from w down to v,
			// recorded in edge order with w repeated at the end.
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
}

// HasCycle reports whether the graph has a cycle (a self-loop or a pair
// of parallel edges counts).
// Complexity is O(1).
func (c *Cycle) HasCycle() bool {
	if c == nil {
		return false
	}
	return len(c.cycle) > 0
}

// Cycle returns a cycle as a list of vertices in edge order with the
// first vertex repeated at the end (a self-loop at v is [v v]; two
// parallel edges v-w are [v w v]; otherwise the cycle is simple), as a
// fresh slice that the caller may mutate.  It returns nil when the
// graph has no cycle (or on a nil receiver).
// Complexity is O(cycle length).
func (c *Cycle) Cycle() []int {
	if !c.HasCycle() {
		return nil
	}
	return append([]int(nil), c.cycle...)
}
