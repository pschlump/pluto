/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph

// DirectedEulerianCycle finds a closed walk that uses every edge of a
// digraph exactly once (Sedgwick's DirectedEulerianCycle, §4.2).  Such a
// cycle exists when the digraph has at least one edge, every vertex's
// in-degree equals its out-degree, and the edge-bearing vertices are
// reachable from one another (a balanced digraph that is connected on
// its nonzero-degree vertices is strongly connected on them); the
// constructor checks the balance and verifies connectivity by checking
// that the walk consumed every edge.  It is an immutable snapshot of the
// digraph at construction time: later AddEdge calls on the digraph are
// not reflected.
type DirectedEulerianCycle struct {
	cycle []int // in edge order, first vertex repeated at the end; nil when none
}

// NewDirectedEulerianCycle searches g for a directed Eulerian cycle via
// Hierholzer's algorithm.  A nil or edgeless digraph has none — there is
// a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewDirectedEulerianCycle(g *Digraph) *DirectedEulerianCycle {
	e := &DirectedEulerianCycle{}
	if g == nil || g.e == 0 {
		return e
	}
	// Necessary condition: in-degree equals out-degree at every vertex.
	for v, ws := range g.adj {
		if g.indeg[v] != len(ws) {
			return e
		}
	}
	cycle := directedWalk(g.adj, firstNonIsolated(g.adj))
	if len(cycle) != g.e+1 {
		return e // the walk did not reach every edge (not strongly connected)
	}
	e.cycle = cycle
	return e
}

// DirectedEulerianPath finds a walk that uses every edge of a digraph
// exactly once (Sedgwick's DirectedEulerianPath, §4.2), open at both
// ends.  Such a path exists when the digraph has at least one edge, at
// most one vertex has out-degree one above its in-degree (the start),
// at most one has in-degree one above its out-degree (the end), and
// every other vertex is balanced.  It is an immutable snapshot of the
// digraph at construction time: later AddEdge calls on the digraph are
// not reflected.
type DirectedEulerianPath struct {
	path []int // in edge order; nil when none
}

// NewDirectedEulerianPath searches g for a directed Eulerian path.  A
// nil or edgeless digraph has none — there is a sane answer, so nothing
// panics.
// Complexity is O(V+E) time, O(V) space.
func NewDirectedEulerianPath(g *Digraph) *DirectedEulerianPath {
	p := &DirectedEulerianPath{}
	if g == nil || g.e == 0 {
		return p
	}
	// Necessary condition: at most one vertex with a surplus out-edge
	// (the start).  Because the totals balance, this also bounds the
	// surplus in-edge vertices (the end) to at most one.
	deficit := 0
	s := -1
	for v, ws := range g.adj {
		if d := len(ws) - g.indeg[v]; d > 0 {
			deficit += d
			s = v
		}
	}
	if deficit > 1 {
		return p
	}
	if s == -1 {
		s = firstNonIsolated(g.adj) // balanced: any edge-bearing vertex
	}
	path := directedWalk(g.adj, s)
	if len(path) != g.e+1 {
		return p // the walk did not reach every edge
	}
	p.path = path
	return p
}

// directedWalk runs Hierholzer's algorithm from s over adjacency lists
// where every entry is a distinct directed edge instance: it follows
// unused out-edges until stuck, backtracks along the vertex stack, and
// emits each vertex the moment it becomes stuck.  The emission order
// reversed is the walk order (for a cycle it starts and ends at s).  The
// adjacency lists are read through per-vertex cursors and never mutated.
func directedWalk(adj [][]int, s int) []int {
	cursor := make([]int, len(adj)) // consumed prefix of each adjacency list
	var stack []int
	var out []int // stuck-vertex emission order
	stack = append(stack, s)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for cursor[v] < len(adj[v]) {
			stack = append(stack, v)
			w := adj[v][cursor[v]]
			cursor[v]++
			v = w
		}
		out = append(out, v)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// firstNonIsolated returns the smallest vertex with at least one
// out-edge (adj must be nonempty overall — the caller guarantees at
// least one edge).
func firstNonIsolated(adj [][]int) int {
	for v, ws := range adj {
		if len(ws) > 0 {
			return v
		}
	}
	return 0
}

// HasEulerianCycle reports whether the digraph has a directed Eulerian
// cycle (nil and edgeless digraphs do not).
// Complexity is O(1).
func (e *DirectedEulerianCycle) HasEulerianCycle() bool {
	if e == nil {
		return false
	}
	return len(e.cycle) > 0
}

// Cycle returns the Eulerian cycle as a list of vertices in edge order
// with the first vertex repeated at the end, as a fresh slice that the
// caller may mutate.  It returns nil when no Eulerian cycle exists (or
// on a nil receiver).
// Complexity is O(cycle length).
func (e *DirectedEulerianCycle) Cycle() []int {
	if !e.HasEulerianCycle() {
		return nil
	}
	return append([]int(nil), e.cycle...)
}

// HasEulerianPath reports whether the digraph has a directed Eulerian
// path (nil and edgeless digraphs do not).
// Complexity is O(1).
func (p *DirectedEulerianPath) HasEulerianPath() bool {
	if p == nil {
		return false
	}
	return len(p.path) > 0
}

// Path returns the Eulerian path as a list of vertices in edge order
// (start to end, E+1 vertices), as a fresh slice that the caller may
// mutate.  It returns nil when no Eulerian path exists (or on a nil
// receiver).
// Complexity is O(path length).
func (p *DirectedEulerianPath) Path() []int {
	if !p.HasEulerianPath() {
		return nil
	}
	return append([]int(nil), p.path...)
}
