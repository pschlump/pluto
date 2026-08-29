/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package digraph_ts

// Topological computes a topological order of a digraph (Sedgwick's
// Topological, §4.5), built on the cycle search and the depth-first
// orders: the reverse postorder is a topological order exactly when the
// digraph has no directed cycle.  It is an immutable snapshot of the
// digraph at construction time: later AddEdge calls on the digraph are
// not reflected, and a Topological is safe for concurrent reads.
type Topological struct {
	order []int // topological order; nil when the digraph has a cycle
}

// NewTopological snapshots the digraph's adjacency lists under the read
// lock, then computes a topological order lock-free on the snapshot.  If
// the digraph has a directed cycle there is no order (HasOrder is false
// and Order is nil).  A nil or empty digraph has no vertices to order —
// HasOrder is false and Order is nil — so nothing panics.
// Complexity is O(V+E) time, O(V) space.
func NewTopological(g *Digraph) *Topological {
	t := &Topological{}
	if g == nil {
		return t
	}
	g.lock.RLock()
	if g.n == 0 {
		g.lock.RUnlock()
		return t
	}
	adj := g.snapshotAdj()
	g.lock.RUnlock()
	var c DirectedCycle
	c.find(adj)
	if c.HasCycle() {
		return t
	}
	var o DepthFirstOrder
	o.dfsAll(adj)
	t.order = o.ReversePost()
	return t
}

// HasOrder reports whether the digraph has a topological order (i.e. it
// is a non-empty DAG).
// Complexity is O(1).
func (t *Topological) HasOrder() bool {
	if t == nil {
		return false
	}
	return len(t.order) > 0
}

// Order returns a topological order of the vertices — for every edge
// v->w, v precedes w — as a fresh slice that the caller may mutate.  It
// returns nil when the digraph has a cycle (or on a nil receiver).
// Complexity is O(V).
func (t *Topological) Order() []int {
	if !t.HasOrder() {
		return nil
	}
	return append([]int(nil), t.order...)
}
