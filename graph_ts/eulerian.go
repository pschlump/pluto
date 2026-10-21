/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package graph_ts

// EulerianCycle finds a closed walk that uses every edge of a graph
// exactly once (Sedgwick's EulerianCycle, §4.1).  Such a cycle exists
// when the graph has at least one edge, every vertex has even degree,
// and the edge-bearing vertices are connected; the constructor checks
// the parities and verifies connectivity by checking that the walk
// consumed every edge.  It is an immutable snapshot of the graph at
// construction time: later AddEdge calls on the graph are not reflected,
// and an EulerianCycle is safe for concurrent reads.
type EulerianCycle struct {
	cycle []int // in edge order, first vertex repeated at the end; nil when none
}

// NewEulerianCycle snapshots the graph's adjacency lists under the read
// lock, then searches the snapshot lock-free for an Eulerian cycle via
// Hierholzer's algorithm.  A nil or edgeless graph has none — there is
// a sane answer, so nothing panics.
// Complexity is O(V+E) time, O(V+E) space.
func NewEulerianCycle(g *Graph) *EulerianCycle {
	e := &EulerianCycle{}
	if g == nil {
		return e
	}
	g.lock.RLock()
	if g.e == 0 {
		g.lock.RUnlock()
		return e
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()

	// Necessary condition: every vertex has even degree (self-loops
	// count twice, which is even).
	eCount := 0
	for _, ws := range adj {
		if len(ws)%2 != 0 {
			return e
		}
		eCount += len(ws)
	}
	eCount /= 2 // each undirected edge appears in two adjacency lists
	cycle := eulerianWalk(buildEdgeView(adj), firstNonIsolated(adj))
	if len(cycle) != eCount+1 {
		return e // the walk did not reach every edge (not connected)
	}
	e.cycle = cycle
	return e
}

// EulerianPath finds a walk that uses every edge of a graph exactly
// once (Sedgwick's EulerianPath, §4.1), open at both ends.  Such a path
// exists when the graph has at least one edge and exactly zero or two
// vertices have odd degree (the odd pair, when present, are the
// endpoints).  It is an immutable snapshot of the graph at construction
// time: later AddEdge calls on the graph are not reflected, and an
// EulerianPath is safe for concurrent reads.
type EulerianPath struct {
	path []int // in edge order; nil when none
}

// NewEulerianPath snapshots the graph's adjacency lists under the read
// lock, then searches the snapshot lock-free for an Eulerian path.  A
// nil or edgeless graph has none — there is a sane answer, so nothing
// panics.
// Complexity is O(V+E) time, O(V+E) space.
func NewEulerianPath(g *Graph) *EulerianPath {
	p := &EulerianPath{}
	if g == nil {
		return p
	}
	g.lock.RLock()
	if g.e == 0 {
		g.lock.RUnlock()
		return p
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()

	// Necessary condition: zero or two odd-degree vertices.
	odd, eCount := 0, 0
	for _, ws := range adj {
		if len(ws)%2 != 0 {
			odd++
		}
		eCount += len(ws)
	}
	eCount /= 2 // each undirected edge appears in two adjacency lists
	if odd != 0 && odd != 2 {
		return p
	}
	// The walk starts at an odd-degree vertex when one exists (paths
	// begin and end at the odd pair), else at any edge-bearing vertex
	// (the graph has an Eulerian cycle).
	s := -1
	for v, ws := range adj {
		if len(ws) == 0 {
			continue
		}
		if s == -1 {
			s = v
		}
		if len(ws)%2 != 0 {
			s = v
			break
		}
	}
	path := eulerianWalk(buildEdgeView(adj), s)
	if len(path) != eCount+1 {
		return p // the walk did not reach every edge (not connected)
	}
	p.path = path
	return p
}

// ueEdge is the local view of an undirected edge: one instance shared
// by both endpoints' lists (by the same vertex twice for a self-loop),
// consumed at most once by the walk.
type ueEdge struct {
	v, w int
	used bool
}

// other returns the edge's endpoint other than v (v itself for a
// self-loop).
func (e *ueEdge) other(v int) int {
	if e.v == v {
		return e.w
	}
	return e.v
}

// buildEdgeView converts adjacency lists (where an undirected edge v-w
// appears once in adj[v] and once in adj[w], and a self-loop at v twice
// in adj[v]) into per-vertex lists of shared edge objects — exactly one
// object per graph edge, in exactly two list slots.  Parallel edges
// become distinct objects.
func buildEdgeView(adj [][]int) [][]*ueEdge {
	local := make([][]*ueEdge, len(adj))
	for v := 0; v < len(adj); v++ {
		selfLoops := 0
		for _, w := range adj[v] {
			if w == v {
				// adj[v] holds a self-loop twice per AddEdge call: make
				// one shared object from each pair of entries.
				if selfLoops%2 == 0 {
					e := &ueEdge{v: v, w: w}
					local[v] = append(local[v], e)
					local[w] = append(local[w], e)
				}
				selfLoops++
			} else if v < w {
				e := &ueEdge{v: v, w: w}
				local[v] = append(local[v], e)
				local[w] = append(local[w], e)
			}
		}
	}
	return local
}

// eulerianWalk runs Hierholzer's algorithm from s over the shared-edge
// view: it follows unused edges until stuck, backtracks along the vertex
// stack, and emits each vertex the moment it becomes stuck.  The
// emission order reversed is the walk order (for a cycle it starts and
// ends at s).
func eulerianWalk(local [][]*ueEdge, s int) []int {
	cursor := make([]int, len(local)) // consumed prefix of each vertex list
	var stack []int
	var out []int // stuck-vertex emission order
	stack = append(stack, s)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for {
			// take the next unused edge at v, if any
			var e *ueEdge
			for cursor[v] < len(local[v]) {
				e = local[v][cursor[v]]
				cursor[v]++
				if !e.used {
					break
				}
				e = nil
			}
			if e == nil {
				break // v is stuck: every remaining edge at v is used
			}
			e.used = true
			stack = append(stack, v)
			v = e.other(v)
		}
		out = append(out, v)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// firstNonIsolated returns the smallest vertex with at least one
// incident edge (adj must have at least one edge overall — the caller
// guarantees a nonzero edge count).
func firstNonIsolated(adj [][]int) int {
	for v, ws := range adj {
		if len(ws) > 0 {
			return v
		}
	}
	return 0
}

// HasEulerianCycle reports whether the graph has an Eulerian cycle (nil
// and edgeless graphs do not).
// Complexity is O(1).
func (e *EulerianCycle) HasEulerianCycle() bool {
	if e == nil {
		return false
	}
	return len(e.cycle) > 0
}

// Cycle returns the Eulerian cycle as a list of vertices in edge order
// with the first vertex repeated at the end (a self-loop at v is
// [v v]; two parallel edges v-w are [v w v]), as a fresh slice that the
// caller may mutate.  It returns nil when no Eulerian cycle exists (or
// on a nil receiver).
// Complexity is O(cycle length).
func (e *EulerianCycle) Cycle() []int {
	if !e.HasEulerianCycle() {
		return nil
	}
	return append([]int(nil), e.cycle...)
}

// HasEulerianPath reports whether the graph has an Eulerian path (nil
// and edgeless graphs do not).  A graph with an Eulerian cycle has an
// Eulerian path too (the cycle itself is a path).
// Complexity is O(1).
func (p *EulerianPath) HasEulerianPath() bool {
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
func (p *EulerianPath) Path() []int {
	if !p.HasEulerianPath() {
		return nil
	}
	return append([]int(nil), p.path...)
}
