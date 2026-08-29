package bellman_ford_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/pschlump/pluto/dijkstra_ts"
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

// tinyEWDAGData is Sedgwick's tinyEWDAG.txt (§4.4), embedded verbatim
// from https://algs4.cs.princeton.edu/44sp/tinyEWDAG.txt — 8 vertices,
// 13 edges, negative weights, no cycle.
const tinyEWDAGData = `
8
13
5 4  0.35
4 7  0.37
5 7  0.28
5 1  0.32
4 0  0.38
0 2  0.26
3 7  0.39
1 3  0.29
7 2  0.34
6 2 -1.20
3 6  0.52
6 0 -1.40
6 4 -1.25
`

// parseDigraph parses an algs4-format edge-weighted digraph dataset
// ("V", "E", then E lines of "from to weight").
func parseDigraph(t *testing.T, data string) *dijkstra_ts.EdgeWeightedDigraph {
	t.Helper()
	r := strings.NewReader(data)
	var nv, ne int
	if _, err := fmt.Fscan(r, &nv, &ne); err != nil {
		t.Fatalf("Failed to parse dataset header: %v", err)
	}
	g := dijkstra_ts.NewEdgeWeightedDigraph(nv)
	for range ne {
		var e dijkstra_ts.DirectedEdge
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

// -------------------------------------------------------------------------------------------------------
// Known answers (mirroring the bellman_ford package tests)
// -------------------------------------------------------------------------------------------------------

// TestBellmanFordTinyEWD checks Bellman-Ford on tinyEWD against the
// known answers (the values the algs4 Java clients print).
func TestBellmanFordTinyEWD(t *testing.T) {
	sp := NewBellmanFordSP(parseDigraph(t, tinyEWDData), 0)

	wantDist := []float64{0, 1.05, 0.26, 0.99, 0.38, 0.73, 1.51, 0.60}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
		if !sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = false, expected true", v)
		}
	}
	if sp.HasNegativeCycle() || sp.NegativeCycle() != nil {
		t.Errorf("Expected no negative cycle in tinyEWD.")
	}
	// PathTo(6) is 0->2->7->3->6 with weight 1.51.
	path := sp.PathTo(6)
	if len(path) != 4 || path[0].From != 0 || path[len(path)-1] != (dijkstra_ts.DirectedEdge{From: 3, To: 6, Weight: 0.52}) {
		t.Errorf("PathTo(6) = %v, expected 0->2->7->3->6", path)
	}
}

// TestBellmanFordTinyEWDn checks Bellman-Ford on tinyEWDn — negative
// edge weights, no negative cycle — against the expected distances from
// source 0 listed on the algs4 booksite
// (https://algs4.cs.princeton.edu/44sp/): 0, 0.93, 0.26, 0.99, 0.26,
// 0.61, 1.51, 0.60.
func TestBellmanFordTinyEWDn(t *testing.T) {
	sp := NewBellmanFordSP(parseDigraph(t, tinyEWDnData), 0)

	wantDist := []float64{0, 0.93, 0.26, 0.99, 0.26, 0.61, 1.51, 0.60}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
	}
	if sp.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true, expected false for tinyEWDn")
	}
}

// negCycleGraph builds a small digraph with a negative cycle reachable
// from 0: 0->1 (1), and the cycle 1->2 (1), 2->3 (1), 3->1 (-4) with
// total weight -2.
func negCycleGraph() *dijkstra_ts.EdgeWeightedDigraph {
	g := dijkstra_ts.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 1, To: 2, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 3, To: 1, Weight: -4})
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
	sum := 0.0
	for i, e := range cycle {
		if next := cycle[(i+1)%len(cycle)]; e.To != next.From {
			t.Errorf("NegativeCycle() is not a cycle at edge %d (%v then %v)", i, e, next)
		}
		sum += e.Weight
	}
	if sum != -2 {
		t.Errorf("NegativeCycle() weights sum to %v, expected -2", sum)
	}
}

// TestBellmanFordNegativeCyclePanics checks the DistTo/PathTo panic
// contract when a negative cycle is reachable from the source.
func TestBellmanFordNegativeCyclePanics(t *testing.T) {
	sp := NewBellmanFordSP(negCycleGraph(), 0)
	expectPanic(t, "DistTo with a negative cycle", "bellman_ford_ts: DistTo", func() { sp.DistTo(1) })
	expectPanic(t, "DistTo with a negative cycle", "negative cycle", func() { sp.DistTo(3) })
	expectPanic(t, "PathTo with a negative cycle", "bellman_ford_ts: PathTo", func() { sp.PathTo(1) })
	expectPanic(t, "PathTo with a negative cycle", "negative cycle", func() { sp.PathTo(2) })
}

// TestBellmanFordUnreachableNegativeCycle: a negative cycle that is NOT
// reachable from the source must not be reported.
func TestBellmanFordUnreachableNegativeCycle(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 3, To: 2, Weight: -2}) // unreachable cycle 2->3->2, weight -1
	sp := NewBellmanFordSP(g, 0)
	if sp.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true, expected false (the cycle is unreachable from 0)")
	}
	if sp.DistTo(1) != 2 {
		t.Errorf("DistTo(1) = %v, expected 2", sp.DistTo(1))
	}
}

// -------------------------------------------------------------------------------------------------------
// AcyclicSP / AcyclicLP known answers
// -------------------------------------------------------------------------------------------------------

// TestAcyclicSPTinyEWDAG checks AcyclicSP from source 5 on tinyEWDAG
// against the known answers (verified with an independent
// relax-everything reference before embedding).
func TestAcyclicSPTinyEWDAG(t *testing.T) {
	g := parseDigraph(t, tinyEWDAGData)
	sp := NewAcyclicSP(g, 5)

	wantDist := []float64{-0.27, 0.32, -0.07, 0.61, -0.12, 0, 1.13, 0.25}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
	}
	// A DAG has no negative cycle, so Bellman-Ford must agree exactly.
	bf := NewBellmanFordSP(g, 5)
	for v := range 8 {
		if !almostEqual(sp.DistTo(v), bf.DistTo(v)) {
			t.Errorf("DistTo(%d) = %v, BellmanFordSP says %v", v, sp.DistTo(v), bf.DistTo(v))
		}
	}
}

// TestAcyclicLPTinyEWDAG checks AcyclicLP from source 5 on tinyEWDAG
// against the known answers (verified with an independent
// enumerate-all-paths reference before embedding).
func TestAcyclicLPTinyEWDAG(t *testing.T) {
	lp := NewAcyclicLP(parseDigraph(t, tinyEWDAGData), 5)

	wantDist := []float64{0.73, 0.32, 1.34, 0.61, 0.35, 0, 1.13, 1.00}
	for v, want := range wantDist {
		if !almostEqual(lp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, lp.DistTo(v), want)
		}
	}
	// Unreachable is -Inf for longest paths (from source 2 below).
	g := parseDigraph(t, tinyEWDAGData)
	lp2 := NewAcyclicLP(g, 2)
	if !math.IsInf(lp2.DistTo(5), -1) || lp2.HasPathTo(5) || lp2.PathTo(5) != nil {
		t.Errorf("Expected vertex 5 (upstream of 2) to report -Inf/false/nil.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Panic contract and nil tolerance
// -------------------------------------------------------------------------------------------------------

func TestBellmanFordTSPanics(t *testing.T) {
	var nilGraph *dijkstra_ts.EdgeWeightedDigraph
	expectPanic(t, "NewBellmanFordSP on a nil graph", "bellman_ford_ts: NewBellmanFordSP", func() {
		NewBellmanFordSP(nilGraph, 0)
	})
	expectPanic(t, "NewBellmanFordSP on a zero-value graph", "empty graph", func() {
		NewBellmanFordSP(&dijkstra_ts.EdgeWeightedDigraph{}, 0)
	})
	expectPanic(t, "NewBellmanFordSP with out-of-range source", "out-of-range source", func() {
		NewBellmanFordSP(dijkstra_ts.NewEdgeWeightedDigraph(3), 3)
	})

	expectPanic(t, "NewAcyclicSP on a nil graph", "bellman_ford_ts: NewAcyclicSP", func() {
		NewAcyclicSP(nilGraph, 0)
	})
	expectPanic(t, "NewAcyclicSP on a zero-value graph", "empty graph", func() {
		NewAcyclicSP(&dijkstra_ts.EdgeWeightedDigraph{}, 0)
	})
	expectPanic(t, "NewAcyclicSP with out-of-range source", "out-of-range source", func() {
		NewAcyclicSP(dijkstra_ts.NewEdgeWeightedDigraph(3), -1)
	})
	expectPanic(t, "NewAcyclicLP on a nil graph", "bellman_ford_ts: NewAcyclicLP", func() {
		NewAcyclicLP(nilGraph, 0)
	})

	// tinyEWD has a cycle (4->5->4): the DAG constructors must say so.
	expectPanic(t, "NewAcyclicSP on tinyEWD", "digraph with a cycle", func() {
		NewAcyclicSP(parseDigraph(t, tinyEWDData), 0)
	})
	expectPanic(t, "NewAcyclicLP on tinyEWD", "digraph with a cycle", func() {
		NewAcyclicLP(parseDigraph(t, tinyEWDData), 0)
	})
}

func TestBellmanFordTSNilTolerated(t *testing.T) {
	var nilSP *BellmanFordSP
	if !math.IsInf(nilSP.DistTo(0), 1) || nilSP.HasPathTo(0) || nilSP.PathTo(0) != nil {
		t.Errorf("Expected a nil BellmanFordSP to report +Inf/false/nil.")
	}
	if nilSP.HasNegativeCycle() || nilSP.NegativeCycle() != nil {
		t.Errorf("Expected a nil BellmanFordSP to report no negative cycle.")
	}
	nilSP.Lock()
	nilSP.Unlock()

	var nilASP *AcyclicSP
	if !math.IsInf(nilASP.DistTo(0), 1) || nilASP.HasPathTo(0) || nilASP.PathTo(0) != nil {
		t.Errorf("Expected a nil AcyclicSP to report +Inf/false/nil.")
	}
	nilASP.Lock()
	nilASP.Unlock()

	var nilLP *AcyclicLP
	if !math.IsInf(nilLP.DistTo(0), -1) || nilLP.HasPathTo(0) || nilLP.PathTo(0) != nil {
		t.Errorf("Expected a nil AcyclicLP to report -Inf/false/nil.")
	}
	nilLP.Lock()
	nilLP.Unlock()
}

// TestBellmanFordTSImmutableResult verifies that a query object is a
// snapshot: edges added after construction are not reflected.
func TestBellmanFordTSImmutableResult(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 10})
	sp := NewBellmanFordSP(g, 0)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 2, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 1, Weight: 1})
	if sp.DistTo(1) != 10 {
		t.Errorf("DistTo(1) = %v, expected 10 (snapshot semantics)", sp.DistTo(1))
	}
}
