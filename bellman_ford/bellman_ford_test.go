package bellman_ford

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
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
// (the algs4 datasets' decimal weights are not exactly representable in
// float64).
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// tinyEWDData is Sedgwick's tinyEWD.txt (§4.4), embedded verbatim from
// https://algs4.cs.princeton.edu/44sp/tinyEWD.txt — 8 vertices, 15
// edges, all non-negative.
const tinyEWDData = `
8
15
4 5  0.35
5 4  0.35
4 7  0.37
5 7  0.28
7 5  0.28
5 1  0.32
0 4  0.38
0 2  0.26
7 3  0.39
1 3  0.29
2 7  0.34
6 2  0.40
3 6  0.52
6 0  0.58
6 4  0.93
`

// tinyEWDnData is Sedgwick's tinyEWDn.txt, embedded verbatim from
// https://algs4.cs.princeton.edu/44sp/tinyEWDn.txt — tinyEWD with the
// three edges leaving vertex 6 negated: it has negative edge weights
// but NO negative cycle.
const tinyEWDnData = `
8
15
4 5  0.35
5 4  0.35
4 7  0.37
5 7  0.28
7 5  0.28
5 1  0.32
0 4  0.38
0 2  0.26
7 3  0.39
1 3  0.29
2 7  0.34
6 2 -1.20
3 6  0.52
6 0 -1.40
6 4 -1.25
`

// parseDigraph parses an algs4-format edge-weighted digraph dataset
// ("V", "E", then E lines of "from to weight").
func parseDigraph(t *testing.T, data string) *dijkstra.EdgeWeightedDigraph {
	t.Helper()
	r := strings.NewReader(data)
	var nv, ne int
	if _, err := fmt.Fscan(r, &nv, &ne); err != nil {
		t.Fatalf("Failed to parse dataset header: %v", err)
	}
	g := dijkstra.NewEdgeWeightedDigraph(nv)
	for range ne {
		var e dijkstra.DirectedEdge
		if _, err := fmt.Fscan(r, &e.From, &e.To, &e.Weight); err != nil {
			t.Fatalf("Failed to parse an edge: %v", err)
		}
		g.AddEdge(e)
	}
	if g.E() != ne {
		t.Fatalf("Expected %d edges after parsing, got %d", ne, g.E())
	}
	return g
}

func tinyEWD(t *testing.T) *dijkstra.EdgeWeightedDigraph  { return parseDigraph(t, tinyEWDData) }
func tinyEWDn(t *testing.T) *dijkstra.EdgeWeightedDigraph { return parseDigraph(t, tinyEWDnData) }

// checkPaths verifies for every reachable v that PathTo(v) starts at s,
// ends at v, is connected, and its weights sum (to tolerance) to
// DistTo(v).
func checkPaths(t *testing.T, sp *BellmanFordSP, s, n int) {
	t.Helper()
	for v := range n {
		if !sp.HasPathTo(v) {
			if sp.PathTo(v) != nil {
				t.Errorf("PathTo(%d) = %v, expected nil for an unreachable vertex", v, sp.PathTo(v))
			}
			continue
		}
		path := sp.PathTo(v)
		if v == s {
			if path == nil || len(path) != 0 {
				t.Errorf("PathTo(%d) = %v, expected an empty non-nil slice", v, path)
			}
			continue
		}
		if len(path) == 0 || path[0].From != s || path[len(path)-1].To != v {
			t.Errorf("PathTo(%d) = %v — wrong endpoints", v, path)
			continue
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

// -------------------------------------------------------------------------------------------------------
// Known answers: tinyEWD (non-negative) and tinyEWDn (negative edges, no negative cycle)
// -------------------------------------------------------------------------------------------------------

// TestBellmanFordTinyEWD checks Bellman-Ford on tinyEWD against the
// known answers (the values the algs4 Java clients print — the same
// known answers dijkstra's tests use) and against pluto/dijkstra itself,
// which must agree on any digraph with non-negative weights.
func TestBellmanFordTinyEWD(t *testing.T) {
	g := tinyEWD(t)
	sp := NewBellmanFordSP(g, 0)

	wantDist := []float64{0, 1.05, 0.26, 0.99, 0.38, 0.73, 1.51, 0.60}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
		if !sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = false, expected true", v)
		}
	}
	if sp.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true, expected false for tinyEWD")
	}
	if sp.NegativeCycle() != nil {
		t.Errorf("NegativeCycle() = %v, expected nil for tinyEWD", sp.NegativeCycle())
	}
	checkPaths(t, sp, 0, 8)

	// Cross-check against the Dijkstra query object: same graph, same
	// distances, exactly (both sum the same decimal weights — agreement
	// to tolerance is what the representation allows).
	dsp := dijkstra.NewDijkstraSP(g, 0)
	for v := range 8 {
		if !almostEqual(sp.DistTo(v), dsp.DistTo(v)) {
			t.Errorf("DistTo(%d) = %v, Dijkstra says %v", v, sp.DistTo(v), dsp.DistTo(v))
		}
	}
}

// TestBellmanFordTinyEWDn checks Bellman-Ford on tinyEWDn — negative
// edge weights, no negative cycle — against the expected shortest-path
// distances from source 0 listed on the algs4 booksite
// (https://algs4.cs.princeton.edu/44sp/, the BellmanFordSP client
// output): 0, 0.93, 0.26, 0.99, 0.26, 0.61, 1.51, 0.60.  (Independently
// verified with a relax-everything reference before embedding.)
func TestBellmanFordTinyEWDn(t *testing.T) {
	g := tinyEWDn(t)
	sp := NewBellmanFordSP(g, 0)

	wantDist := []float64{0, 0.93, 0.26, 0.99, 0.26, 0.61, 1.51, 0.60}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
	}
	if sp.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true, expected false for tinyEWDn (negative edges but no negative cycle)")
	}

	// The known shortest-path-tree edges, last edge per vertex (algs4's
	// client output paths: 0->2->7->3->6->4->5->1 and prefixes).
	wantLastEdge := map[int]dijkstra.DirectedEdge{
		1: {From: 5, To: 1, Weight: 0.32},
		2: {From: 0, To: 2, Weight: 0.26},
		3: {From: 7, To: 3, Weight: 0.39},
		4: {From: 6, To: 4, Weight: -1.25},
		5: {From: 4, To: 5, Weight: 0.35},
		6: {From: 3, To: 6, Weight: 0.52},
		7: {From: 2, To: 7, Weight: 0.34},
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
	checkPaths(t, sp, 0, 8)
}

// TestBellmanFordNegativeEdgesRequired double-checks the point of the
// package: the same tinyEWDn digraph is rejected by Dijkstra's
// constructor (negative weights) but answered by Bellman-Ford.
func TestBellmanFordNegativeEdgesRequired(t *testing.T) {
	expectPanic(t, "NewDijkstraSP on tinyEWDn", "negative weight", func() {
		dijkstra.NewDijkstraSP(tinyEWDn(t), 0)
	})
	sp := NewBellmanFordSP(tinyEWDn(t), 0)
	if !almostEqual(sp.DistTo(4), 0.26) {
		t.Errorf("DistTo(4) = %v, expected 0.26 via the negative edge 6->4", sp.DistTo(4))
	}
}

// -------------------------------------------------------------------------------------------------------
// Negative-cycle detection and the DistTo/PathTo panic contract
// -------------------------------------------------------------------------------------------------------

// negCycleGraph builds a small digraph with a negative cycle reachable
// from 0: 0->1 (1), and the cycle 1->2 (1), 2->3 (1), 3->1 (-4) with
// total weight -2.
func negCycleGraph() *dijkstra.EdgeWeightedDigraph {
	g := dijkstra.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 1, Weight: -4})
	return g
}

func TestBellmanFordNegativeCycle(t *testing.T) {
	sp := NewBellmanFordSP(negCycleGraph(), 0)
	if !sp.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = false, expected true")
	}
	cycle := sp.NegativeCycle()
	if len(cycle) != 3 {
		t.Fatalf("NegativeCycle() = %v, expected the 3-edge cycle", cycle)
	}
	// The edges form a closed directed cycle with negative total weight.
	sum := 0.0
	for i, e := range cycle {
		if next := cycle[(i+1)%len(cycle)]; e.To != next.From {
			t.Errorf("NegativeCycle() is not a cycle at edge %d (%v then %v)", i, e, next)
		}
		sum += e.Weight
	}
	if sum >= 0 {
		t.Errorf("NegativeCycle() weights sum to %v, expected a negative total", sum)
	}
	if sum != -2 {
		t.Errorf("NegativeCycle() weights sum to %v, expected -2", sum)
	}
}

// TestBellmanFordNegativeCyclePanics checks the algs4 semantics: when a
// negative cycle is reachable from the source, DistTo and PathTo are
// undefined and panic with a message naming the method and the cycle.
func TestBellmanFordNegativeCyclePanics(t *testing.T) {
	sp := NewBellmanFordSP(negCycleGraph(), 0)
	expectPanic(t, "DistTo with a negative cycle", "bellman_ford: DistTo", func() { sp.DistTo(1) })
	expectPanic(t, "DistTo with a negative cycle", "negative cycle", func() { sp.DistTo(3) })
	expectPanic(t, "PathTo with a negative cycle", "bellman_ford: PathTo", func() { sp.PathTo(1) })
	expectPanic(t, "PathTo with a negative cycle", "negative cycle", func() { sp.PathTo(2) })
}

// TestBellmanFordNegativeSelfLoop: a negative self-loop is a negative
// cycle of one edge.
func TestBellmanFordNegativeSelfLoop(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(2)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 1, Weight: -0.5})
	sp := NewBellmanFordSP(g, 0)
	if !sp.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = false, expected true for a negative self-loop")
	}
	cycle := sp.NegativeCycle()
	if len(cycle) != 1 || cycle[0] != (dijkstra.DirectedEdge{From: 1, To: 1, Weight: -0.5}) {
		t.Errorf("NegativeCycle() = %v, expected the single self-loop edge", cycle)
	}
}

// TestBellmanFordUnreachableNegativeCycle: a negative cycle that is NOT
// reachable from the source must not be reported — shortest paths from
// the source are still well defined.
func TestBellmanFordUnreachableNegativeCycle(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 2, Weight: -2}) // cycle 2->3->2, weight -1, unreachable from 0
	sp := NewBellmanFordSP(g, 0)
	if sp.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true, expected false (the cycle is unreachable from 0)")
	}
	if sp.NegativeCycle() != nil {
		t.Errorf("NegativeCycle() = %v, expected nil", sp.NegativeCycle())
	}
	if sp.DistTo(1) != 2 {
		t.Errorf("DistTo(1) = %v, expected 2", sp.DistTo(1))
	}
	if !math.IsInf(sp.DistTo(2), 1) || sp.HasPathTo(2) {
		t.Errorf("DistTo(2)/HasPathTo(2) = %v/%v, expected +Inf/false", sp.DistTo(2), sp.HasPathTo(2))
	}
}

// -------------------------------------------------------------------------------------------------------
// Unreachable vertices, panic contract, nil tolerance, snapshots
// -------------------------------------------------------------------------------------------------------

// TestBellmanFordUnreachable checks the +Inf / false / nil reporting for
// unreachable and out-of-range vertices (dijkstra's conventions).
func TestBellmanFordUnreachable(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	sp := NewBellmanFordSP(g, 0)

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
	// Source-to-source is an empty non-nil slice.
	if path := sp.PathTo(0); path == nil || len(path) != 0 {
		t.Errorf("PathTo(0) = %v, expected an empty non-nil slice", path)
	}
}

func TestBellmanFordPanics(t *testing.T) {
	var nilGraph *dijkstra.EdgeWeightedDigraph
	expectPanic(t, "NewBellmanFordSP on a nil graph", "bellman_ford: NewBellmanFordSP", func() {
		NewBellmanFordSP(nilGraph, 0)
	})
	expectPanic(t, "NewBellmanFordSP on a zero-value graph", "nil or empty graph", func() {
		NewBellmanFordSP(&dijkstra.EdgeWeightedDigraph{}, 0)
	})
	expectPanic(t, "NewBellmanFordSP with out-of-range source", "out-of-range source", func() {
		NewBellmanFordSP(dijkstra.NewEdgeWeightedDigraph(3), 3)
	})
	expectPanic(t, "NewBellmanFordSP with negative source", "out-of-range source", func() {
		NewBellmanFordSP(dijkstra.NewEdgeWeightedDigraph(3), -1)
	})
}

// TestBellmanFordNilTolerated verifies that a nil query object reports
// +Inf/false/nil for every question.
func TestBellmanFordNilTolerated(t *testing.T) {
	var nilSP *BellmanFordSP
	if !math.IsInf(nilSP.DistTo(0), 1) || nilSP.HasPathTo(0) || nilSP.PathTo(0) != nil {
		t.Errorf("Expected a nil BellmanFordSP to report +Inf/false/nil.")
	}
	if nilSP.HasNegativeCycle() || nilSP.NegativeCycle() != nil {
		t.Errorf("Expected a nil BellmanFordSP to report no negative cycle.")
	}
	nilSP.Lock()
	nilSP.Unlock()
}

// TestBellmanFordImmutableResult verifies that a query object is a
// snapshot: edges added after construction are not reflected.
func TestBellmanFordImmutableResult(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 10})
	sp := NewBellmanFordSP(g, 0)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 1, Weight: 1}) // a much cheaper 0->2->1 route
	if sp.DistTo(1) != 10 {
		t.Errorf("DistTo(1) = %v, expected 10 (snapshot semantics)", sp.DistTo(1))
	}
}
