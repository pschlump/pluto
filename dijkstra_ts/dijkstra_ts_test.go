package dijkstra_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

// expectPanic runs fx and fails the test unless it panics; when want is
// non-empty the panic message must contain it.
func expectPanic(t *testing.T, name, want string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
			return
		}
		if want != "" {
			if msg, ok := r.(string); !ok || !strings.Contains(msg, want) {
				t.Errorf("Unexpected panic message from %s: %v (expected it to contain %q)", name, r, want)
			}
		}
	}()
	fx()
}

// almostEqual reports whether a and b agree to within a small tolerance
// (the tinyEWD weights are not exactly representable in float64).
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// tinyEWD builds Sedgwick's classic tinyEWD.txt digraph (§4.4): 8
// vertices, 15 edges.  The known shortest-path tree from source 0 is
// verified in TestDijkstraSPTinyEWD.
func tinyEWD() *EdgeWeightedDigraph {
	g := NewEdgeWeightedDigraph(8)
	for _, e := range []DirectedEdge{
		{4, 5, 0.35},
		{5, 4, 0.35},
		{4, 7, 0.37},
		{5, 7, 0.28},
		{7, 5, 0.28},
		{5, 1, 0.32},
		{0, 4, 0.38},
		{0, 2, 0.26},
		{7, 3, 0.39},
		{1, 3, 0.29},
		{2, 7, 0.34},
		{6, 2, 0.40},
		{3, 6, 0.52},
		{6, 0, 0.58},
		{6, 4, 0.93},
	} {
		g.AddEdge(e)
	}
	return g
}

// tinyGraph builds a small undirected graph with integer weights so the
// known answers compare exactly:
//
//	0--1 (2), 1--2 (3), 0--2 (10), 2--3 (1), 3--4 (1), 0--4 (20), 1--3 (8)
//
// Shortest distances from 0: 0, 2, 5, 6, 7.
func tinyGraph() *EdgeWeightedGraph {
	g := NewEdgeWeightedGraph(5)
	for _, e := range []Edge{
		{0, 1, 2},
		{1, 2, 3},
		{0, 2, 10},
		{2, 3, 1},
		{3, 4, 1},
		{0, 4, 20},
		{1, 3, 8},
	} {
		g.AddEdge(e)
	}
	return g
}

// -------------------------------------------------------------------------------------------------------
// Graph types: constructors, counts, adjacency
// -------------------------------------------------------------------------------------------------------

func TestNewEdgeWeightedDigraph(t *testing.T) {
	g := NewEdgeWeightedDigraph(8)
	if g.V() != 8 || g.E() != 0 {
		t.Errorf("Expected a new digraph with V=8, E=0, got V=%d, E=%d", g.V(), g.E())
	}
	for v := range 8 {
		if adj := g.Adj(v); adj != nil {
			t.Errorf("Expected Adj(%d) of a new digraph to be nil, got %v", v, adj)
		}
	}
	if g.Adj(-1) != nil || g.Adj(8) != nil {
		t.Errorf("Expected Adj on out-of-range vertices to be nil.")
	}
}

func TestDigraphAddEdge(t *testing.T) {
	g := tinyEWD()
	if g.V() != 8 || g.E() != 15 {
		t.Errorf("Expected tinyEWD with V=8, E=15, got V=%d, E=%d", g.V(), g.E())
	}
	if adj := g.Adj(0); len(adj) != 2 || adj[0] != (DirectedEdge{0, 4, 0.38}) || adj[1] != (DirectedEdge{0, 2, 0.26}) {
		t.Errorf("Adj(0) = %v, expected the two edges leaving 0 in insertion order", adj)
	}

	// Out-of-range endpoints report false and change nothing.
	if g.AddEdge(DirectedEdge{0, 8, 1}) || g.AddEdge(DirectedEdge{-1, 0, 1}) || g.AddEdge(DirectedEdge{8, 0, 1}) {
		t.Errorf("Expected AddEdge with an out-of-range endpoint to report false.")
	}
	if g.E() != 15 {
		t.Errorf("Out-of-range AddEdge changed the graph: E = %d", g.E())
	}
}

func TestDigraphSelfLoopAndParallelEdges(t *testing.T) {
	g := NewEdgeWeightedDigraph(3)
	g.AddEdge(DirectedEdge{0, 0, 5}) // self-loop
	g.AddEdge(DirectedEdge{0, 1, 9}) // parallel edges: the cheap one wins
	g.AddEdge(DirectedEdge{0, 1, 4})
	if g.E() != 3 {
		t.Errorf("Expected E=3 (self-loop and parallels each count), got %d", g.E())
	}
	if len(g.Adj(0)) != 3 {
		t.Errorf("Expected Adj(0) to hold all 3 edges, got %v", g.Adj(0))
	}

	sp := NewDijkstraSP(g, 0)
	if sp.DistTo(0) != 0 { // a positive self-loop is never an improvement
		t.Errorf("DistTo(0) = %v, expected 0 (self-loop ignored)", sp.DistTo(0))
	}
	if sp.DistTo(1) != 4 {
		t.Errorf("DistTo(1) = %v, expected 4 (cheapest parallel edge)", sp.DistTo(1))
	}
}

func TestNewEdgeWeightedGraph(t *testing.T) {
	g := tinyGraph()
	if g.V() != 5 || g.E() != 7 {
		t.Errorf("Expected tinyGraph with V=5, E=7, got V=%d, E=%d", g.V(), g.E())
	}
	// Undirected: each edge is in both endpoints' lists.
	if len(g.Adj(0)) != 3 { // 0--1, 0--2, 0--4
		t.Errorf("Expected Adj(0) to hold 3 edges, got %v", g.Adj(0))
	}
	if len(g.Adj(4)) != 2 { // 3--4, 0--4
		t.Errorf("Expected Adj(4) to hold 2 edges, got %v", g.Adj(4))
	}
	if g.AddEdge(Edge{0, 5, 1}) {
		t.Errorf("Expected AddEdge with an out-of-range endpoint to report false.")
	}
}

func TestGraphSelfLoopCounts(t *testing.T) {
	g := NewEdgeWeightedGraph(2)
	g.AddEdge(Edge{0, 0, 5})
	if g.E() != 1 {
		t.Errorf("Expected a self-loop to count once in E, got %d", g.E())
	}
	if len(g.Adj(0)) != 2 { // twice in the adjacency list, like graph's Degree
		t.Errorf("Expected a self-loop to appear twice in Adj(0), got %v", g.Adj(0))
	}
}

func TestEdgeOther(t *testing.T) {
	e := Edge{V: 3, W: 7, Weight: 1.5}
	if e.Other(3) != 7 || e.Other(7) != 3 {
		t.Errorf("Other returned the wrong endpoint.")
	}
	// A self-loop: both endpoints are v.
	if (Edge{V: 2, W: 2, Weight: 1}).Other(2) != 2 {
		t.Errorf("Other on a self-loop should return v.")
	}
}

// -------------------------------------------------------------------------------------------------------
// DijkstraSP: known answers on tinyEWD
// -------------------------------------------------------------------------------------------------------

// TestDijkstraSPTinyEWD checks the shortest-path tree from source 0 of
// Sedgwick's tinyEWD against the known answers (the same values the
// algs4 Java client prints).
func TestDijkstraSPTinyEWD(t *testing.T) {
	sp := NewDijkstraSP(tinyEWD(), 0)

	wantDist := []float64{0, 1.05, 0.26, 0.99, 0.38, 0.73, 1.51, 0.60}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
		if !sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = false, expected true", v)
		}
	}

	// The known shortest-path tree edges, parent-first per vertex.
	wantLastEdge := map[int]DirectedEdge{
		1: {5, 1, 0.32},
		2: {0, 2, 0.26},
		3: {7, 3, 0.39},
		4: {0, 4, 0.38},
		5: {4, 5, 0.35},
		6: {3, 6, 0.52},
		7: {2, 7, 0.34},
	}
	for v, want := range wantLastEdge {
		path := sp.PathTo(v)
		if len(path) == 0 {
			t.Errorf("PathTo(%d) returned no edges.", v)
			continue
		}
		if last := path[len(path)-1]; last != want {
			t.Errorf("PathTo(%d) ends with %v, expected %v", v, last, want)
		}
	}

	// Source-to-source is an empty non-nil slice.
	if path := sp.PathTo(0); path == nil || len(path) != 0 {
		t.Errorf("PathTo(0) = %v, expected an empty non-nil slice", path)
	}
}

// TestDijkstraSPPath checks that a reconstructed path starts at the
// source, ends at v, is connected, and its weights sum to DistTo(v).
func TestDijkstraSPPath(t *testing.T) {
	sp := NewDijkstraSP(tinyEWD(), 0)
	for v := 1; v < 8; v++ {
		path := sp.PathTo(v)
		if path[0].From != 0 {
			t.Errorf("PathTo(%d) starts at %d, expected the source 0", v, path[0].From)
		}
		if path[len(path)-1].To != v {
			t.Errorf("PathTo(%d) ends at %d, expected %d", v, path[len(path)-1].To, v)
		}
		sum := 0.0
		for i, e := range path {
			if i > 0 && path[i-1].To != e.From {
				t.Errorf("PathTo(%d) is not connected at edge %d (%v after %v)", v, i, e, path[i-1])
			}
			sum += e.Weight
		}
		if !almostEqual(sum, sp.DistTo(v)) {
			t.Errorf("PathTo(%d) weights sum to %v, DistTo(%d) = %v", v, sum, v, sp.DistTo(v))
		}
	}
}

// TestDijkstraSPUnreachable checks the +Inf / false / nil reporting for
// unreachable and out-of-range vertices.
func TestDijkstraSPUnreachable(t *testing.T) {
	g := NewEdgeWeightedDigraph(4)
	g.AddEdge(DirectedEdge{0, 1, 2})
	sp := NewDijkstraSP(g, 0)

	for _, v := range []int{2, 3} {
		if !math.IsInf(sp.DistTo(v), 1) {
			t.Errorf("DistTo(%d) = %v, expected +Inf", v, sp.DistTo(v))
		}
		if sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = true, expected false", v)
		}
		if sp.PathTo(v) != nil {
			t.Errorf("PathTo(%d) = %v, expected nil", v, sp.PathTo(v))
		}
	}
	for _, bad := range []int{-1, 4, 100} {
		if !math.IsInf(sp.DistTo(bad), 1) || sp.HasPathTo(bad) || sp.PathTo(bad) != nil {
			t.Errorf("Out-of-range vertex %d did not report +Inf/false/nil.", bad)
		}
	}
}

// TestDijkstraSPSourceNotZero checks a source other than 0, including
// vertices unreachable "upstream" in tinyEWD.
func TestDijkstraSPSourceNotZero(t *testing.T) {
	sp := NewDijkstraSP(tinyEWD(), 6)
	// From 6: 6->0 (0.58), 6->2 (0.40), 6->4 (0.93), and onwards.
	if !almostEqual(sp.DistTo(0), 0.58) || !almostEqual(sp.DistTo(2), 0.40) {
		t.Errorf("DistTo(0)/DistTo(2) from 6 = %v/%v, expected 0.58/0.40", sp.DistTo(0), sp.DistTo(2))
	}
	// 6->2->7->5->1 = 0.40+0.34+0.28+0.32 = 1.34
	if !almostEqual(sp.DistTo(1), 1.34) {
		t.Errorf("DistTo(1) from 6 = %v, expected 1.34", sp.DistTo(1))
	}
	if !sp.HasPathTo(4) {
		t.Errorf("Expected 4 to be reachable from 6.")
	}
}

// -------------------------------------------------------------------------------------------------------
// DijkstraUndirectedSP: known answers on tinyGraph
// -------------------------------------------------------------------------------------------------------

func TestDijkstraUndirectedSP(t *testing.T) {
	sp := NewDijkstraUndirectedSP(tinyGraph(), 0)

	wantDist := []float64{0, 2, 5, 6, 7}
	for v, want := range wantDist {
		if sp.DistTo(v) != want { // integer weights: exact comparison
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
		if !sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = false, expected true", v)
		}
	}

	// Path 0 -> 4 is 0--1--2--3--4 with total weight 7.
	path := sp.PathTo(4)
	if len(path) != 4 {
		t.Fatalf("PathTo(4) = %v, expected 4 edges", path)
	}
	sum := 0.0
	at := 0
	for _, e := range path {
		if e.Other(at) == at {
			t.Errorf("PathTo(4) edge %v does not advance from %d", e, at)
		}
		at = e.Other(at)
		sum += e.Weight
	}
	if at != 4 {
		t.Errorf("PathTo(4) ends at %d, expected 4", at)
	}
	if sum != sp.DistTo(4) {
		t.Errorf("PathTo(4) weights sum to %v, DistTo(4) = %v", sum, sp.DistTo(4))
	}

	if path := sp.PathTo(0); path == nil || len(path) != 0 {
		t.Errorf("PathTo(0) = %v, expected an empty non-nil slice", path)
	}
}

// TestDijkstraUndirectedSPSelfLoop checks that a positive self-loop is
// never part of a shortest path, and that parallel edges pick the cheap
// one.
func TestDijkstraUndirectedSPSelfLoop(t *testing.T) {
	g := NewEdgeWeightedGraph(3)
	g.AddEdge(Edge{0, 0, 5})
	g.AddEdge(Edge{0, 1, 9})
	g.AddEdge(Edge{0, 1, 4})
	sp := NewDijkstraUndirectedSP(g, 0)
	if sp.DistTo(0) != 0 || sp.DistTo(1) != 4 {
		t.Errorf("DistTo(0)/DistTo(1) = %v/%v, expected 0/4", sp.DistTo(0), sp.DistTo(1))
	}
	if !math.IsInf(sp.DistTo(2), 1) {
		t.Errorf("DistTo(2) = %v, expected +Inf (isolated vertex)", sp.DistTo(2))
	}
}

// -------------------------------------------------------------------------------------------------------
// DijkstraAllPairsSP
// -------------------------------------------------------------------------------------------------------

func TestDijkstraAllPairsSP(t *testing.T) {
	g := tinyEWD()
	ap := NewDijkstraAllPairsSP(g)

	// Agrees with the single-source query object from every source.
	for s := 0; s < 8; s++ {
		sp := NewDijkstraSP(g, s)
		for v := 0; v < 8; v++ {
			if ap.Dist(s, v) != sp.DistTo(v) {
				t.Errorf("Dist(%d,%d) = %v, single-source DistTo = %v", s, v, ap.Dist(s, v), sp.DistTo(v))
			}
			if ap.HasPath(s, v) != sp.HasPathTo(v) {
				t.Errorf("HasPath(%d,%d) disagrees with HasPathTo.", s, v)
			}
		}
	}

	// A spot-checked path and its weight sum.
	path := ap.Path(0, 6)
	if len(path) != 4 { // 0->2->7->3->6
		t.Fatalf("Path(0,6) = %v, expected 4 edges", path)
	}
	sum := 0.0
	for _, e := range path {
		sum += e.Weight
	}
	if !almostEqual(sum, ap.Dist(0, 6)) {
		t.Errorf("Path(0,6) weights sum to %v, Dist(0,6) = %v", sum, ap.Dist(0, 6))
	}
	if path := ap.Path(3, 3); path == nil || len(path) != 0 {
		t.Errorf("Path(3,3) = %v, expected an empty non-nil slice", path)
	}

	// Out-of-range vertices report +Inf / false / nil.
	if !math.IsInf(ap.Dist(0, 8), 1) || !math.IsInf(ap.Dist(-1, 0), 1) {
		t.Errorf("Expected +Inf for out-of-range Dist.")
	}
	if ap.HasPath(8, 0) || ap.Path(0, -1) != nil {
		t.Errorf("Expected false/nil for out-of-range HasPath/Path.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Panic contract
// -------------------------------------------------------------------------------------------------------

func TestDijkstraPanics(t *testing.T) {
	expectPanic(t, "NewEdgeWeightedDigraph(0)", "NewEdgeWeightedDigraph", func() { NewEdgeWeightedDigraph(0) })
	expectPanic(t, "NewEdgeWeightedDigraph(-2)", "v >= 1", func() { NewEdgeWeightedDigraph(-2) })
	expectPanic(t, "NewEdgeWeightedGraph(0)", "NewEdgeWeightedGraph", func() { NewEdgeWeightedGraph(0) })

	var nilDigraph *EdgeWeightedDigraph
	expectPanic(t, "AddEdge on a nil digraph", "AddEdge", func() {
		nilDigraph.AddEdge(DirectedEdge{0, 1, 1})
	})
	var nilGraph *EdgeWeightedGraph
	expectPanic(t, "AddEdge on a nil graph", "AddEdge", func() {
		nilGraph.AddEdge(Edge{0, 1, 1})
	})

	expectPanic(t, "NewDijkstraSP on a nil graph", "NewDijkstraSP", func() { NewDijkstraSP(nilDigraph, 0) })
	expectPanic(t, "NewDijkstraSP on a zero-value graph", "NewDijkstraSP", func() {
		NewDijkstraSP(&EdgeWeightedDigraph{}, 0)
	})
	expectPanic(t, "NewDijkstraSP with out-of-range source", "out-of-range source", func() {
		NewDijkstraSP(NewEdgeWeightedDigraph(3), 3)
	})
	expectPanic(t, "NewDijkstraUndirectedSP on a nil graph", "NewDijkstraUndirectedSP", func() {
		NewDijkstraUndirectedSP(nilGraph, 0)
	})
	expectPanic(t, "NewDijkstraUndirectedSP with out-of-range source", "out-of-range source", func() {
		NewDijkstraUndirectedSP(NewEdgeWeightedGraph(3), -1)
	})
	expectPanic(t, "NewDijkstraAllPairsSP on a nil graph", "NewDijkstraAllPairsSP", func() {
		NewDijkstraAllPairsSP(nilDigraph)
	})
}

// TestDijkstraNegativeWeightPanics checks that the constructors reject
// negative edge weights — Dijkstra's algorithm is only correct for
// non-negative weights.
func TestDijkstraNegativeWeightPanics(t *testing.T) {
	expectPanic(t, "NewDijkstraSP with a negative edge", "negative weight", func() {
		g := NewEdgeWeightedDigraph(3)
		g.AddEdge(DirectedEdge{0, 1, 2})
		g.AddEdge(DirectedEdge{1, 2, -0.5})
		NewDijkstraSP(g, 0)
	})
	// Even an unreachable negative edge is rejected (validation is upfront).
	expectPanic(t, "NewDijkstraSP with an unreachable negative edge", "1->2", func() {
		g := NewEdgeWeightedDigraph(3)
		g.AddEdge(DirectedEdge{1, 2, -0.5})
		NewDijkstraSP(g, 0)
	})
	expectPanic(t, "NewDijkstraUndirectedSP with a negative edge", "negative weight", func() {
		g := NewEdgeWeightedGraph(2)
		g.AddEdge(Edge{0, 1, -1})
		NewDijkstraUndirectedSP(g, 0)
	})
	expectPanic(t, "NewDijkstraAllPairsSP with a negative edge", "negative weight", func() {
		g := NewEdgeWeightedDigraph(2)
		g.AddEdge(DirectedEdge{0, 1, -1})
		NewDijkstraAllPairsSP(g)
	})
}

// -------------------------------------------------------------------------------------------------------
// Nil and zero-value tolerance
// -------------------------------------------------------------------------------------------------------

// TestDijkstraNilTolerated verifies that every read treats a nil
// receiver as an empty graph / a "no paths" result.
func TestDijkstraNilTolerated(t *testing.T) {
	var nilDigraph *EdgeWeightedDigraph
	if nilDigraph.V() != 0 || nilDigraph.E() != 0 || nilDigraph.Adj(0) != nil {
		t.Errorf("Expected a nil digraph to behave as an empty graph.")
	}
	nilDigraph.Lock()   // no-op, must not panic
	nilDigraph.Unlock() // no-op, must not panic

	var nilGraph *EdgeWeightedGraph
	if nilGraph.V() != 0 || nilGraph.E() != 0 || nilGraph.Adj(0) != nil {
		t.Errorf("Expected a nil graph to behave as an empty graph.")
	}
	nilGraph.Lock()
	nilGraph.Unlock()

	var nilSP *DijkstraSP
	if !math.IsInf(nilSP.DistTo(0), 1) || nilSP.HasPathTo(0) || nilSP.PathTo(0) != nil {
		t.Errorf("Expected a nil DijkstraSP to report +Inf/false/nil.")
	}
	nilSP.Lock()
	nilSP.Unlock()

	var nilUSP *DijkstraUndirectedSP
	if !math.IsInf(nilUSP.DistTo(0), 1) || nilUSP.HasPathTo(0) || nilUSP.PathTo(0) != nil {
		t.Errorf("Expected a nil DijkstraUndirectedSP to report +Inf/false/nil.")
	}
	nilUSP.Lock()
	nilUSP.Unlock()

	var nilAP *DijkstraAllPairsSP
	if !math.IsInf(nilAP.Dist(0, 0), 1) || nilAP.HasPath(0, 0) || nilAP.Path(0, 0) != nil {
		t.Errorf("Expected a nil DijkstraAllPairsSP to report +Inf/false/nil.")
	}
	nilAP.Lock()
	nilAP.Unlock()
}

// TestDijkstraZeroValue verifies that zero-value graphs behave as empty
// graphs: AddEdge reports false (every endpoint is out of range when
// there are no vertices) and the reads report empty.
func TestDijkstraZeroValue(t *testing.T) {
	var g EdgeWeightedDigraph
	if g.V() != 0 || g.E() != 0 || g.Adj(0) != nil {
		t.Errorf("Expected a zero-value digraph to behave as an empty graph.")
	}
	if g.AddEdge(DirectedEdge{0, 0, 1}) {
		t.Errorf("Expected AddEdge on a zero-value digraph to report false.")
	}
	g.Lock()
	g.Unlock()

	var ug EdgeWeightedGraph
	if ug.V() != 0 || ug.E() != 0 || ug.Adj(0) != nil {
		t.Errorf("Expected a zero-value graph to behave as an empty graph.")
	}
	if ug.AddEdge(Edge{0, 0, 1}) {
		t.Errorf("Expected AddEdge on a zero-value graph to report false.")
	}
	ug.Lock()
	ug.Unlock()
}

// TestDijkstraImmutableResult verifies that a query object is a snapshot:
// edges added after construction are not reflected.
func TestDijkstraImmutableResult(t *testing.T) {
	g := NewEdgeWeightedDigraph(3)
	g.AddEdge(DirectedEdge{0, 1, 10})
	sp := NewDijkstraSP(g, 0)
	g.AddEdge(DirectedEdge{0, 2, 1})
	g.AddEdge(DirectedEdge{2, 1, 1}) // a much cheaper 0->2->1 route
	if sp.DistTo(1) != 10 {
		t.Errorf("DistTo(1) = %v, expected 10 (snapshot semantics)", sp.DistTo(1))
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

func benchmarkDigraph(n, m int, seed int64) *EdgeWeightedDigraph {
	rng := rand.New(rand.NewSource(seed))
	g := NewEdgeWeightedDigraph(n)
	for range m {
		g.AddEdge(DirectedEdge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(100) + 1)})
	}
	return g
}

func BenchmarkDijkstraSP(b *testing.B) {
	g := benchmarkDigraph(4096, 16384, 42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewDijkstraSP(g, 0)
	}
}
