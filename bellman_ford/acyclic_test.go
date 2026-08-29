package bellman_ford

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// tinyDAG builds a small hand-made edge-weighted DAG with integer
// weights (including a negative one) so the known answers compare
// exactly:
//
//	0->1 (2), 0->2 (5), 1->2 (-3), 1->3 (4), 2->3 (1), 3->4 (2), 0->4 (20)
//
// Vertex 5 is isolated (unreachable from 0).
//
// Shortest distances from 0: 0, 2, -1, 0, 2.
// Longest distances from 0:  0, 2, 5, 6, 20.
func tinyDAG() *dijkstra.EdgeWeightedDigraph {
	g := dijkstra.NewEdgeWeightedDigraph(6)
	for _, e := range []dijkstra.DirectedEdge{
		{From: 0, To: 1, Weight: 2},
		{From: 0, To: 2, Weight: 5},
		{From: 1, To: 2, Weight: -3},
		{From: 1, To: 3, Weight: 4},
		{From: 2, To: 3, Weight: 1},
		{From: 3, To: 4, Weight: 2},
		{From: 0, To: 4, Weight: 20},
	} {
		g.AddEdge(e)
	}
	return g
}

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

// -------------------------------------------------------------------------------------------------------
// AcyclicSP
// -------------------------------------------------------------------------------------------------------

// TestAcyclicSPTinyDAG checks shortest paths on the hand-made DAG
// against the known answers — exact, integer weights.
func TestAcyclicSPTinyDAG(t *testing.T) {
	sp := NewAcyclicSP(tinyDAG(), 0)

	wantDist := []float64{0, 2, -1, 0, 2}
	for v, want := range wantDist {
		if sp.DistTo(v) != want {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
		if !sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = false, expected true", v)
		}
	}
	// Vertex 5 is isolated.
	if !math.IsInf(sp.DistTo(5), 1) || sp.HasPathTo(5) || sp.PathTo(5) != nil {
		t.Errorf("Expected the isolated vertex 5 to report +Inf/false/nil, got %v/%v/%v",
			sp.DistTo(5), sp.HasPathTo(5), sp.PathTo(5))
	}
	// The shortest 0 -> 4 path is 0->1->2->3->4 with weight 2.
	path := sp.PathTo(4)
	if len(path) != 4 {
		t.Fatalf("PathTo(4) = %v, expected 4 edges", path)
	}
	sum := 0.0
	for _, e := range path {
		sum += e.Weight
	}
	if sum != sp.DistTo(4) {
		t.Errorf("PathTo(4) weights sum to %v, DistTo(4) = %v", sum, sp.DistTo(4))
	}
	if path := sp.PathTo(0); path == nil || len(path) != 0 {
		t.Errorf("PathTo(0) = %v, expected an empty non-nil slice", path)
	}
}

// TestAcyclicSPTinyEWDAG checks AcyclicSP from source 5 on Sedgwick's
// tinyEWDAG against the known answers (verified with an independent
// relax-everything reference before embedding; the negative edges make
// several distances negative).
func TestAcyclicSPTinyEWDAG(t *testing.T) {
	g := parseDigraph(t, tinyEWDAGData)
	sp := NewAcyclicSP(g, 5)

	wantDist := []float64{-0.27, 0.32, -0.07, 0.61, -0.12, 0, 1.13, 0.25}
	for v, want := range wantDist {
		if !almostEqual(sp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, sp.DistTo(v), want)
		}
		if !sp.HasPathTo(v) {
			t.Errorf("HasPathTo(%d) = false, expected true (5 reaches everything)", v)
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

// -------------------------------------------------------------------------------------------------------
// AcyclicLP
// -------------------------------------------------------------------------------------------------------

// TestAcyclicLPTinyDAG checks longest paths on the hand-made DAG against
// the known answers.
func TestAcyclicLPTinyDAG(t *testing.T) {
	lp := NewAcyclicLP(tinyDAG(), 0)

	wantDist := []float64{0, 2, 5, 6, 20}
	for v, want := range wantDist {
		if lp.DistTo(v) != want {
			t.Errorf("DistTo(%d) = %v, expected %v", v, lp.DistTo(v), want)
		}
	}
	// Vertex 5 is isolated: longest-path unreachable is -Inf.
	if !math.IsInf(lp.DistTo(5), -1) || lp.HasPathTo(5) || lp.PathTo(5) != nil {
		t.Errorf("Expected the isolated vertex 5 to report -Inf/false/nil, got %v/%v/%v",
			lp.DistTo(5), lp.HasPathTo(5), lp.PathTo(5))
	}
	// The longest 0 -> 4 path is the direct edge 0->4 (weight 20).
	path := lp.PathTo(4)
	if len(path) != 1 || path[0] != (dijkstra.DirectedEdge{From: 0, To: 4, Weight: 20}) {
		t.Errorf("PathTo(4) = %v, expected the direct edge 0->4", path)
	}
	if path := lp.PathTo(0); path == nil || len(path) != 0 {
		t.Errorf("PathTo(0) = %v, expected an empty non-nil slice", path)
	}
}

// TestAcyclicLPTinyEWDAG checks AcyclicLP from source 5 on tinyEWDAG
// against the known answers (verified with an independent
// enumerate-all-paths reference before embedding).
func TestAcyclicLPTinyEWDAG(t *testing.T) {
	g := parseDigraph(t, tinyEWDAGData)
	lp := NewAcyclicLP(g, 5)

	wantDist := []float64{0.73, 0.32, 1.34, 0.61, 0.35, 0, 1.13, 1.00}
	for v, want := range wantDist {
		if !almostEqual(lp.DistTo(v), want) {
			t.Errorf("DistTo(%d) = %v, expected %v", v, lp.DistTo(v), want)
		}
	}
	// The longest 5 -> 2 path is 5->1->3->7->2 (weight 1.34): its last
	// edge is 7->2.
	if path := lp.PathTo(2); len(path) == 0 || path[len(path)-1] != (dijkstra.DirectedEdge{From: 7, To: 2, Weight: 0.34}) {
		t.Errorf("PathTo(2) = %v, expected it to end with 7->2", path)
	}
}

// TestAcyclicLPCriticalPath is a miniature of algs4's critical-path
// client (CPM.java): the longest path from the source schedules every
// job as early as the precedence constraints allow.  Two jobs: job 0
// takes 4, job 1 takes 3 and must follow job 0.
func TestAcyclicLPCriticalPath(t *testing.T) {
	// Vertices: 0 = source, 1 = job0 start, 2 = job0 end, 3 = job1
	// start, 4 = job1 end, 5 = sink.
	g := dijkstra.NewEdgeWeightedDigraph(6)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 0})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 3, Weight: 0})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 4}) // job 0 duration
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 4, Weight: 3}) // job 1 duration
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 0}) // precedence: job 1 after job 0
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 5, Weight: 0})
	g.AddEdge(dijkstra.DirectedEdge{From: 4, To: 5, Weight: 0})

	lp := NewAcyclicLP(g, 0)
	if lp.DistTo(3) != 4 { // job 1 starts when job 0 finishes
		t.Errorf("DistTo(job1 start) = %v, expected 4", lp.DistTo(3))
	}
	if lp.DistTo(5) != 7 { // makespan 4 + 3
		t.Errorf("DistTo(sink) = %v, expected 7", lp.DistTo(5))
	}
}

// -------------------------------------------------------------------------------------------------------
// Panic contract
// -------------------------------------------------------------------------------------------------------

func TestAcyclicPanics(t *testing.T) {
	var nilGraph *dijkstra.EdgeWeightedDigraph
	expectPanic(t, "NewAcyclicSP on a nil graph", "bellman_ford: NewAcyclicSP", func() {
		NewAcyclicSP(nilGraph, 0)
	})
	expectPanic(t, "NewAcyclicSP on a zero-value graph", "nil or empty graph", func() {
		NewAcyclicSP(&dijkstra.EdgeWeightedDigraph{}, 0)
	})
	expectPanic(t, "NewAcyclicSP with out-of-range source", "out-of-range source", func() {
		NewAcyclicSP(tinyDAG(), 6)
	})
	expectPanic(t, "NewAcyclicLP on a nil graph", "bellman_ford: NewAcyclicLP", func() {
		NewAcyclicLP(nilGraph, 0)
	})
	expectPanic(t, "NewAcyclicLP with out-of-range source", "out-of-range source", func() {
		NewAcyclicLP(tinyDAG(), -1)
	})

	// A digraph with a cycle has no topological order: both constructors
	// panic, and the message must say why.
	expectPanic(t, "NewAcyclicSP on a cyclic digraph", "digraph with a cycle", func() {
		g := tinyDAG()
		g.AddEdge(dijkstra.DirectedEdge{From: 4, To: 1, Weight: 1}) // closes 1->2->3->4->1
		NewAcyclicSP(g, 0)
	})
	expectPanic(t, "NewAcyclicSP on tinyEWD (cycle 4->5->4)", "bellman_ford: NewAcyclicSP", func() {
		NewAcyclicSP(tinyEWD(t), 0)
	})
	expectPanic(t, "NewAcyclicLP on a cyclic digraph", "digraph with a cycle", func() {
		g := tinyDAG()
		g.AddEdge(dijkstra.DirectedEdge{From: 4, To: 1, Weight: 1})
		NewAcyclicLP(g, 0)
	})
	// A positive self-loop is already a cycle.
	expectPanic(t, "NewAcyclicSP with a self-loop", "cycle", func() {
		g := dijkstra.NewEdgeWeightedDigraph(2)
		g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 0, Weight: 1})
		NewAcyclicSP(g, 0)
	})
}

// TestAcyclicNilTolerated verifies that nil query objects report
// +Inf/-Inf/false/nil for every question.
func TestAcyclicNilTolerated(t *testing.T) {
	var nilSP *AcyclicSP
	if !math.IsInf(nilSP.DistTo(0), 1) || nilSP.HasPathTo(0) || nilSP.PathTo(0) != nil {
		t.Errorf("Expected a nil AcyclicSP to report +Inf/false/nil.")
	}
	nilSP.Lock()
	nilSP.Unlock()

	var nilLP *AcyclicLP
	if !math.IsInf(nilLP.DistTo(0), -1) || nilLP.HasPathTo(0) || nilLP.PathTo(0) != nil {
		t.Errorf("Expected a nil AcyclicLP to report -Inf/false/nil.")
	}
	nilLP.Lock()
	nilLP.Unlock()
}

// TestAcyclicImmutableResult verifies snapshot semantics for the DAG
// query objects.
func TestAcyclicImmutableResult(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 10})
	sp := NewAcyclicSP(g, 0)
	lp := NewAcyclicLP(g, 0)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 1, Weight: 1}) // cheaper 0->2->1 route, still a DAG
	if sp.DistTo(1) != 10 || lp.DistTo(1) != 10 {
		t.Errorf("DistTo(1) = %v/%v, expected 10/10 (snapshot semantics)", sp.DistTo(1), lp.DistTo(1))
	}
}
