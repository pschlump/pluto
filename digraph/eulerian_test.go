package digraph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// directedEdgeMultiset counts every directed edge instance of g —
// parallel edges are separate entries, a self-loop v→v is one entry.
func directedEdgeMultiset(g *Digraph) map[[2]int]int {
	m := make(map[[2]int]int)
	for v := 0; v < g.V(); v++ {
		for _, w := range adjOf(g, v) {
			m[[2]int{v, w}]++
		}
	}
	return m
}

// verifyDirectedWalk checks that walk is a legal walk over g using every
// edge exactly once (length E+1, every consecutive pair an actual edge
// instance, each instance consumed once).  It consumes m.
func verifyDirectedWalk(t *testing.T, m map[[2]int]int, walk []int, closed bool) {
	t.Helper()
	if closed && (len(walk) == 0 || walk[0] != walk[len(walk)-1]) {
		t.Errorf("Eulerian cycle must start and end at the same vertex, got %v", walk)
	}
	for i := 0; i+1 < len(walk); i++ {
		e := [2]int{walk[i], walk[i+1]}
		if m[e] == 0 {
			t.Errorf("walk step %d→%d is not an edge of the digraph", e[0], e[1])
			return
		}
		m[e]--
	}
	for e, left := range m {
		if left != 0 {
			t.Errorf("edge %v used %d times too few — the walk misses it", e, left)
			return
		}
	}
}

// degreesOf returns the out- and in-degree of v (test-side helper; the
// ok values are irrelevant for in-range vertices).
func degreesOf(g *Digraph, v int) (out, in int) {
	out, _ = g.OutDegree(v)
	in, _ = g.InDegree(v)
	return out, in
}

// weaklyConnectedNonIsolated reports whether every vertex with at least
// one incident edge is reachable from every other such vertex in the
// underlying undirected graph (union-find over the edge set).
func weaklyConnectedNonIsolated(g *Digraph) bool {
	parent := make([]int, g.V())
	for i := range parent {
		parent[i] = i
	}
	find := func(v int) int {
		for parent[v] != v {
			parent[v] = parent[parent[v]]
			v = parent[v]
		}
		return v
	}
	union := func(a, b int) { parent[find(a)] = find(b) }
	for v := 0; v < g.V(); v++ {
		for _, w := range adjOf(g, v) {
			union(v, w)
		}
	}
	root := -1
	for v := 0; v < g.V(); v++ {
		if out, in := degreesOf(g, v); out == 0 && in == 0 {
			continue // isolated vertices are irrelevant to Eulerian walks
		}
		if root == -1 {
			root = find(v)
		} else if find(v) != root {
			return false
		}
	}
	return true
}

// hasDirectedEulerianCycleOracle is the textbook existence condition
// for a directed Eulerian cycle, computed independently of the query:
// at least one edge, in-degree equals out-degree everywhere, and the
// edge-bearing vertices weakly connected (a balanced digraph that is
// weakly connected on them is strongly connected).
func hasDirectedEulerianCycleOracle(g *Digraph) bool {
	if g.E() == 0 || !weaklyConnectedNonIsolated(g) {
		return false
	}
	for v := 0; v < g.V(); v++ {
		if out, in := degreesOf(g, v); out != in {
			return false
		}
	}
	return true
}

// hasDirectedEulerianPathOracle is the textbook existence condition for
// a directed Eulerian path: the cycle condition, or exactly one start
// (out-in = 1) and one end (in-out = 1) with the rest balanced, and the
// edge-bearing vertices weakly connected.
func hasDirectedEulerianPathOracle(g *Digraph) bool {
	if g.E() == 0 || !weaklyConnectedNonIsolated(g) {
		return false
	}
	starts, ends := 0, 0
	for v := 0; v < g.V(); v++ {
		out, in := degreesOf(g, v)
		switch {
		case out == in:
		case out == in+1:
			starts++
		case in == out+1:
			ends++
		default:
			return false
		}
	}
	return starts == 0 && ends == 0 || starts == 1 && ends == 1
}

func TestDirectedEulerianCycleTwoVertices(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	g.AddEdge(1, 0)
	ec := NewDirectedEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle for 0→1→0")
	}
	cycle := ec.Cycle()
	if len(cycle) != 3 || cycle[0] != cycle[2] {
		t.Errorf("Expected [0 1 0], got %v", cycle)
	}
	verifyDirectedWalk(t, directedEdgeMultiset(g), cycle, true)
}

func TestDirectedEulerianCycleSelfLoop(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 0)
	ec := NewDirectedEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle for a lone self-loop")
	}
	if cycle := ec.Cycle(); len(cycle) != 2 || cycle[0] != 0 {
		t.Errorf("Expected [0 0], got %v", cycle)
	}
	// A self-loop is also a (one-edge) path.
	if path := NewDirectedEulerianPath(g).Path(); len(path) != 2 {
		t.Errorf("Expected path [0 0], got %v", path)
	}
}

func TestDirectedEulerianParallelEdges(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	g.AddEdge(0, 1)
	g.AddEdge(1, 0)
	g.AddEdge(1, 0)
	ec := NewDirectedEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle over parallel edges")
	}
	verifyDirectedWalk(t, directedEdgeMultiset(g), ec.Cycle(), true)
}

func TestDirectedEulerianDisconnected(t *testing.T) {
	// Two disjoint directed triangles: balanced, but no single walk can
	// cover both components.
	g := NewDigraph(6)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {3, 4}, {4, 5}, {5, 3}} {
		g.AddEdge(e[0], e[1])
	}
	if ec := NewDirectedEulerianCycle(g); ec.HasEulerianCycle() {
		t.Errorf("Expected no Eulerian cycle on a disconnected digraph")
	}
	if ep := NewDirectedEulerianPath(g); ep.HasEulerianPath() {
		t.Errorf("Expected no Eulerian path on a disconnected digraph")
	}
}

func TestDirectedEulerianPathUnbalanced(t *testing.T) {
	// A directed triangle plus a tail 0→3: no cycle (0 has out 2, in 1),
	// but an Eulerian path exists from 0 to 3.
	g := NewDigraph(4)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {0, 3}} {
		g.AddEdge(e[0], e[1])
	}
	if ec := NewDirectedEulerianCycle(g); ec.HasEulerianCycle() {
		t.Errorf("Expected no Eulerian cycle on an unbalanced digraph")
	}
	ep := NewDirectedEulerianPath(g)
	if !ep.HasEulerianPath() {
		t.Fatalf("Expected an Eulerian path")
	}
	path := ep.Path()
	if len(path) != 5 || path[0] != 0 || path[4] != 3 {
		t.Errorf("Expected a 5-vertex path 0..3, got %v", path)
	}
	verifyDirectedWalk(t, directedEdgeMultiset(g), path, false)
}

func TestDirectedEulerianPathTwoEnds(t *testing.T) {
	// A star 0→1, 0→2 has two deficit ends (1 and 2): no Eulerian path.
	g := NewDigraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(0, 2)
	if ep := NewDirectedEulerianPath(g); ep.HasEulerianPath() {
		t.Errorf("Expected no Eulerian path with two end candidates")
	}
}

func TestDirectedEulerianSnapshot(t *testing.T) {
	g := NewDigraph(2)
	g.AddEdge(0, 1)
	ec := NewDirectedEulerianCycle(g)
	ep := NewDirectedEulerianPath(g)
	// The result objects are snapshots: later edges are not reflected.
	// The one-edge digraph has a path (0→1) but no cycle; adding 1→0
	// afterwards must not change either answer.
	g.AddEdge(1, 0)
	if ec.HasEulerianCycle() {
		t.Errorf("Expected snapshot semantics: later AddEdge must not be reflected")
	}
	if got := ep.Path(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("Expected snapshot path [0 1], got %v", got)
	}
}

// TestDirectedEulerianNilAndEmpty: a nil or edgeless digraph has a sane
// answer — no Eulerian cycle or path (the algs4 convention: an Eulerian
// walk needs at least one edge) — so nothing panics.
func TestDirectedEulerianNilAndEmpty(t *testing.T) {
	var nilGraph *Digraph
	var zeroGraph Digraph
	for _, g := range []*Digraph{nilGraph, &zeroGraph, NewDigraph(5)} {
		if NewDirectedEulerianCycle(g).HasEulerianCycle() {
			t.Errorf("Expected HasEulerianCycle false for nil/edgeless digraph")
		}
		if NewDirectedEulerianCycle(g).Cycle() != nil {
			t.Errorf("Expected Cycle nil for nil/edgeless digraph")
		}
		if NewDirectedEulerianPath(g).HasEulerianPath() {
			t.Errorf("Expected HasEulerianPath false for nil/edgeless digraph")
		}
		if NewDirectedEulerianPath(g).Path() != nil {
			t.Errorf("Expected Path nil for nil/edgeless digraph")
		}
	}
}

func TestDirectedEulerianNilTolerated(t *testing.T) {
	var ec *DirectedEulerianCycle
	var ep *DirectedEulerianPath
	if ec.HasEulerianCycle() || ec.Cycle() != nil {
		t.Errorf("Expected false/nil on nil receiver")
	}
	if ep.HasEulerianPath() || ep.Path() != nil {
		t.Errorf("Expected false/nil on nil receiver")
	}
}

// TestDirectedEulerianRandomized cross-checks both queries against the
// textbook existence conditions on random digraphs and verifies every
// reported walk as a certificate.
func TestDirectedEulerianRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	for step := 0; step < 400; step++ {
		n := 1 + rng.Intn(12)
		g := NewDigraph(n)
		m := rng.Intn(3 * n)
		for i := 0; i < m; i++ {
			g.AddEdge(rng.Intn(n), rng.Intn(n))
		}

		ec := NewDirectedEulerianCycle(g)
		if got, want := ec.HasEulerianCycle(), hasDirectedEulerianCycleOracle(g); got != want {
			t.Fatalf("step %d: HasEulerianCycle=%v, oracle=%v (n=%d m=%d)", step, got, want, n, g.E())
		}
		if ec.HasEulerianCycle() {
			verifyDirectedWalk(t, directedEdgeMultiset(g), ec.Cycle(), true)
		}

		ep := NewDirectedEulerianPath(g)
		if got, want := ep.HasEulerianPath(), hasDirectedEulerianPathOracle(g); got != want {
			t.Fatalf("step %d: HasEulerianPath=%v, oracle=%v (n=%d m=%d)", step, got, want, n, g.E())
		}
		if ep.HasEulerianPath() {
			verifyDirectedWalk(t, directedEdgeMultiset(g), ep.Path(), false)
		}
	}
}
