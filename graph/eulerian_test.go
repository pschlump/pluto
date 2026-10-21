package graph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// undirectedEdgeMultiset counts every undirected edge instance of g —
// parallel edges are separate entries, a self-loop at v is one entry
// (adj[v] holds it twice, so the raw count is halved).
func undirectedEdgeMultiset(g *Graph) map[[2]int]int {
	m := make(map[[2]int]int)
	self := make(map[int]int)
	for v := 0; v < g.V(); v++ {
		for _, w := range adjOf(g, v) {
			if w == v {
				self[v]++
			} else if v < w {
				m[[2]int{v, w}]++
			}
		}
	}
	for v, c := range self {
		m[[2]int{v, v}] = c / 2
	}
	return m
}

// verifyUndirectedWalk checks that walk is a legal walk over g using
// every edge exactly once (length E+1, every consecutive pair an actual
// edge instance, each instance consumed once).  It consumes m.
func verifyUndirectedWalk(t *testing.T, m map[[2]int]int, walk []int, closed bool) {
	t.Helper()
	if closed && (len(walk) == 0 || walk[0] != walk[len(walk)-1]) {
		t.Errorf("Eulerian cycle must start and end at the same vertex, got %v", walk)
	}
	for i := 0; i+1 < len(walk); i++ {
		a, b := walk[i], walk[i+1]
		if b < a {
			a, b = b, a
		}
		e := [2]int{a, b}
		if m[e] == 0 {
			t.Errorf("walk step %d-%d is not an edge of the graph", walk[i], walk[i+1])
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

// hasEulerianCycleOracle is the textbook existence condition for an
// Eulerian cycle, computed independently of the query: at least one
// edge, all degrees even, and the edge-bearing vertices connected (CC
// from this package; isolated vertices sit in their own components).
func hasEulerianCycleOracle(g *Graph) bool {
	if g.E() == 0 {
		return false
	}
	cc := NewCC(g)
	root := -1
	for v := 0; v < g.V(); v++ {
		d, _ := g.Degree(v)
		if d%2 != 0 {
			return false
		}
		if d == 0 {
			continue // isolated vertices are irrelevant to Eulerian walks
		}
		id, _ := cc.ID(v)
		if root == -1 {
			root = id
		} else if id != root {
			return false
		}
	}
	return true
}

// hasEulerianPathOracle is the textbook existence condition for an
// Eulerian path: at least one edge, zero or two odd-degree vertices, and
// the edge-bearing vertices connected.
func hasEulerianPathOracle(g *Graph) bool {
	if g.E() == 0 {
		return false
	}
	odd := 0
	for v := 0; v < g.V(); v++ {
		if d, _ := g.Degree(v); d%2 != 0 {
			odd++
		}
	}
	return (odd == 0 || odd == 2) && hasEulerianCycleConnectivity(g)
}

// hasEulerianCycleConnectivity reports whether the edge-bearing
// vertices all share a connected component.
func hasEulerianCycleConnectivity(g *Graph) bool {
	cc := NewCC(g)
	root := -1
	for v := 0; v < g.V(); v++ {
		if d, _ := g.Degree(v); d == 0 {
			continue
		}
		id, _ := cc.ID(v)
		if root == -1 {
			root = id
		} else if id != root {
			return false
		}
	}
	return true
}

func TestEulerianCycleTriangle(t *testing.T) {
	g := NewGraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)
	g.AddEdge(2, 0)
	ec := NewEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle on the triangle")
	}
	cycle := ec.Cycle()
	if len(cycle) != 4 || cycle[0] != cycle[3] {
		t.Errorf("Expected a 4-vertex closed cycle, got %v", cycle)
	}
	verifyUndirectedWalk(t, undirectedEdgeMultiset(g), cycle, true)
}

func TestEulerianCycleSelfLoop(t *testing.T) {
	g := NewGraph(2)
	g.AddEdge(0, 0)
	ec := NewEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle for a lone self-loop")
	}
	if cycle := ec.Cycle(); len(cycle) != 2 || cycle[0] != 0 {
		t.Errorf("Expected [0 0], got %v", cycle)
	}
	// A self-loop is also a (one-edge) path.
	if path := NewEulerianPath(g).Path(); len(path) != 2 {
		t.Errorf("Expected path [0 0], got %v", path)
	}
}

func TestEulerianParallelEdges(t *testing.T) {
	g := NewGraph(2)
	g.AddEdge(0, 1)
	g.AddEdge(0, 1)
	ec := NewEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle over two parallel edges")
	}
	cycle := ec.Cycle()
	if len(cycle) != 3 || cycle[0] != 0 {
		t.Errorf("Expected [0 1 0], got %v", cycle)
	}
	verifyUndirectedWalk(t, undirectedEdgeMultiset(g), cycle, true)
}

func TestEulerianCycleDisconnected(t *testing.T) {
	// Two disjoint triangles: all degrees even, but no single walk can
	// cover both components.
	g := NewGraph(6)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {3, 4}, {4, 5}, {5, 3}} {
		g.AddEdge(e[0], e[1])
	}
	if ec := NewEulerianCycle(g); ec.HasEulerianCycle() {
		t.Errorf("Expected no Eulerian cycle on a disconnected graph")
	}
	if ep := NewEulerianPath(g); ep.HasEulerianPath() {
		t.Errorf("Expected no Eulerian path on a disconnected graph")
	}
}

func TestEulerianPathOddDegrees(t *testing.T) {
	// The path graph 0-1-2: no cycle (two odd degrees), path 0..2.
	g := NewGraph(3)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)
	if ec := NewEulerianCycle(g); ec.HasEulerianCycle() {
		t.Errorf("Expected no Eulerian cycle with odd degrees")
	}
	ep := NewEulerianPath(g)
	if !ep.HasEulerianPath() {
		t.Fatalf("Expected an Eulerian path")
	}
	path := ep.Path()
	if len(path) != 3 || path[0] != 0 || path[2] != 2 {
		t.Errorf("Expected path [0 1 2], got %v", path)
	}
	verifyUndirectedWalk(t, undirectedEdgeMultiset(g), path, false)
}

func TestEulerianPathFourOdd(t *testing.T) {
	// K4: every degree is 3 — four odd vertices, no Eulerian anything.
	g := NewGraph(4)
	for v := 0; v < 4; v++ {
		for w := v + 1; w < 4; w++ {
			g.AddEdge(v, w)
		}
	}
	if ec := NewEulerianCycle(g); ec.HasEulerianCycle() {
		t.Errorf("Expected no Eulerian cycle on K4")
	}
	if ep := NewEulerianPath(g); ep.HasEulerianPath() {
		t.Errorf("Expected no Eulerian path on K4")
	}
}

func TestEulerianIsolatedVertices(t *testing.T) {
	// A triangle plus isolated vertices: the walk ignores the isolates.
	g := NewGraph(5)
	g.AddEdge(0, 1)
	g.AddEdge(1, 2)
	g.AddEdge(2, 0)
	ec := NewEulerianCycle(g)
	if !ec.HasEulerianCycle() {
		t.Fatalf("Expected an Eulerian cycle ignoring isolated vertices")
	}
	verifyUndirectedWalk(t, undirectedEdgeMultiset(g), ec.Cycle(), true)
}

func TestEulerianSnapshot(t *testing.T) {
	g := NewGraph(2)
	g.AddEdge(0, 1)
	ec := NewEulerianCycle(g)
	ep := NewEulerianPath(g)
	// The result objects are snapshots: later edges are not reflected.
	// The one-edge graph has a path (0-1) but no cycle; adding 1-2
	// afterwards must not change either answer.
	g.AddEdge(1, 2)
	if ec.HasEulerianCycle() {
		t.Errorf("Expected snapshot semantics: later AddEdge must not be reflected")
	}
	if got := ep.Path(); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("Expected snapshot path [0 1], got %v", got)
	}
}

// TestEulerianNilAndEmpty: a nil or edgeless graph has a sane answer —
// no Eulerian cycle or path (the algs4 convention: an Eulerian walk
// needs at least one edge) — so nothing panics.
func TestEulerianNilAndEmpty(t *testing.T) {
	var nilGraph *Graph
	var zeroGraph Graph
	for _, g := range []*Graph{nilGraph, &zeroGraph, NewGraph(5)} {
		if NewEulerianCycle(g).HasEulerianCycle() {
			t.Errorf("Expected HasEulerianCycle false for nil/edgeless graph")
		}
		if NewEulerianCycle(g).Cycle() != nil {
			t.Errorf("Expected Cycle nil for nil/edgeless graph")
		}
		if NewEulerianPath(g).HasEulerianPath() {
			t.Errorf("Expected HasEulerianPath false for nil/edgeless graph")
		}
		if NewEulerianPath(g).Path() != nil {
			t.Errorf("Expected Path nil for nil/edgeless graph")
		}
	}
}

func TestEulerianNilTolerated(t *testing.T) {
	var ec *EulerianCycle
	var ep *EulerianPath
	if ec.HasEulerianCycle() || ec.Cycle() != nil {
		t.Errorf("Expected false/nil on nil receiver")
	}
	if ep.HasEulerianPath() || ep.Path() != nil {
		t.Errorf("Expected false/nil on nil receiver")
	}
}

// TestEulerianRandomized cross-checks both queries against the textbook
// existence conditions on random graphs and verifies every reported
// walk as a certificate.
func TestEulerianRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	for step := 0; step < 400; step++ {
		n := 1 + rng.Intn(12)
		g := NewGraph(n)
		m := rng.Intn(3 * n)
		for i := 0; i < m; i++ {
			g.AddEdge(rng.Intn(n), rng.Intn(n))
		}

		ec := NewEulerianCycle(g)
		if got, want := ec.HasEulerianCycle(), hasEulerianCycleOracle(g); got != want {
			t.Fatalf("step %d: HasEulerianCycle=%v, oracle=%v (n=%d m=%d)", step, got, want, n, g.E())
		}
		if ec.HasEulerianCycle() {
			verifyUndirectedWalk(t, undirectedEdgeMultiset(g), ec.Cycle(), true)
		}

		ep := NewEulerianPath(g)
		if got, want := ep.HasEulerianPath(), hasEulerianPathOracle(g); got != want {
			t.Fatalf("step %d: HasEulerianPath=%v, oracle=%v (n=%d m=%d)", step, got, want, n, g.E())
		}
		if ep.HasEulerianPath() {
			verifyUndirectedWalk(t, undirectedEdgeMultiset(g), ep.Path(), false)
		}
	}
}
