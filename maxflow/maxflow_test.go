package maxflow

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"strconv"
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

// tinyFNData is Sedgwick's classic tinyFN.txt dataset (§6.4), embedded
// verbatim from https://algs4.cs.princeton.edu/64maxflow/tinyFN.txt —
// 6 vertices, 8 edges, documented maxflow value 4 from vertex 0 to
// vertex 5 (cross-checked against the brute-force min-cut oracle in
// thorough_test.go).
const tinyFNData = `6
8
0 1 2.0
0 2 3.0
1 3 3.0
1 4 1.0
2 3 1.0
2 4 1.0
3 5 2.0
4 5 3.0
`

// parseFlowNetwork parses the algs4 flow network format — V, E, then E
// lines of "v w capacity" — and returns the network plus the flat edge
// list (for the brute-force oracle).
func parseFlowNetwork(t *testing.T, data string) (*FlowNetwork, []FlowEdge) {
	t.Helper()
	fields := strings.Fields(data)
	if len(fields) < 2 {
		t.Fatalf("bad network data: %q", data)
	}
	n, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("bad vertex count: %v", err)
	}
	m, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatalf("bad edge count: %v", err)
	}
	if len(fields) != 2+3*m {
		t.Fatalf("expected %d edge fields, got %d", 3*m, len(fields)-2)
	}
	g := NewFlowNetwork(n)
	edges := make([]FlowEdge, 0, m)
	for i := 0; i < m; i++ {
		v, err1 := strconv.Atoi(fields[2+3*i])
		w, err2 := strconv.Atoi(fields[3+3*i])
		cap, err3 := strconv.ParseFloat(fields[4+3*i], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatalf("bad edge %d: %v %v %v", i, err1, err2, err3)
		}
		if !g.AddEdge(v, w, cap) {
			t.Fatalf("edge %d (%d->%d) rejected", i, v, w)
		}
		edges = append(edges, FlowEdge{From: v, To: w, Capacity: cap})
	}
	return g, edges
}

// -------------------------------------------------------------------------------------------------------
// Known answer on tinyFN
// -------------------------------------------------------------------------------------------------------

func TestTinyFN(t *testing.T) {
	g, edges := parseFlowNetwork(t, tinyFNData)
	if g.V() != 6 || g.E() != 8 {
		t.Fatalf("V/E = %d/%d, expected 6/8", g.V(), g.E())
	}
	ff := NewFordFulkerson(g, 0, 5)
	if ff.Value() != 4 {
		t.Errorf("tinyFN maxflow = %v, expected 4 (the documented algs4 value)", ff.Value())
	}
	// The documented value must also agree with the fully independent
	// brute-force min-cut oracle.
	if want := minCutBruteForce(6, edges, 0, 5); want != 4 {
		t.Errorf("brute-force oracle says tinyFN min cut = %v, expected 4", want)
	}
	checkMaxflow(t, "tinyFN", g, ff)
}

// -------------------------------------------------------------------------------------------------------
// Edge cases
// -------------------------------------------------------------------------------------------------------

// TestNoPath checks that with no s -> t path the maxflow is 0 and the
// whole source component is on the source side of the cut.
func TestNoPath(t *testing.T) {
	g := NewFlowNetwork(4)
	g.AddEdge(0, 1, 5)
	g.AddEdge(2, 3, 7) // a disconnected component
	ff := NewFordFulkerson(g, 0, 3)
	if ff.Value() != 0 {
		t.Errorf("Value = %v, expected 0 (no path 0 -> 3)", ff.Value())
	}
	if !ff.InMinCut(0) || !ff.InMinCut(1) || ff.InMinCut(2) || ff.InMinCut(3) {
		t.Errorf("unexpected min cut side: 0:%v 1:%v 2:%v 3:%v",
			ff.InMinCut(0), ff.InMinCut(1), ff.InMinCut(2), ff.InMinCut(3))
	}
	checkMaxflow(t, "no-path", g, ff)
}

// TestDirectParallelEdges checks that parallel edges each carry flow.
func TestDirectParallelEdges(t *testing.T) {
	g := NewFlowNetwork(2)
	g.AddEdge(0, 1, 2)
	g.AddEdge(0, 1, 3)
	g.AddEdge(0, 1, 0) // a zero-capacity parallel edge
	ff := NewFordFulkerson(g, 0, 1)
	if ff.Value() != 5 {
		t.Errorf("Value = %v, expected 5", ff.Value())
	}
	checkMaxflow(t, "parallel", g, ff)
}

// TestSelfLoopIgnored checks that a self-loop, however capacious, never
// carries useful flow.
func TestSelfLoopIgnored(t *testing.T) {
	g := NewFlowNetwork(3)
	g.AddEdge(0, 0, 100) // self-loop on the source
	g.AddEdge(0, 1, 4)
	g.AddEdge(1, 1, 100) // self-loop on an intermediate vertex
	g.AddEdge(1, 2, 3)
	ff := NewFordFulkerson(g, 0, 2)
	if ff.Value() != 3 {
		t.Errorf("Value = %v, expected 3", ff.Value())
	}
	checkMaxflow(t, "self-loop", g, ff)
	for _, e := range ff.Edges() {
		if e.From == e.To && e.Flow != 0 {
			t.Errorf("self-loop %d->%d carries flow %v", e.From, e.To, e.Flow)
		}
	}
}

// TestZeroCapacityEdges checks that zero-capacity edges pass no flow.
func TestZeroCapacityEdges(t *testing.T) {
	g := NewFlowNetwork(3)
	g.AddEdge(0, 1, 0)
	g.AddEdge(1, 2, 5)
	ff := NewFordFulkerson(g, 0, 2)
	if ff.Value() != 0 {
		t.Errorf("Value = %v, expected 0", ff.Value())
	}
	checkMaxflow(t, "zero-capacity", g, ff)
}

// TestBackwardEdge exercises flow cancellation: the max flow must route
// one unit 0->2->1->3, canceling the naive 1->2 assignment.
func TestBackwardEdge(t *testing.T) {
	g := NewFlowNetwork(4)
	g.AddEdge(0, 1, 1)
	g.AddEdge(0, 2, 1)
	g.AddEdge(1, 2, 1) // the cross edge the algorithm must sometimes cancel
	g.AddEdge(1, 3, 1)
	g.AddEdge(2, 3, 1)
	ff := NewFordFulkerson(g, 0, 3)
	if ff.Value() != 2 {
		t.Errorf("Value = %v, expected 2", ff.Value())
	}
	checkMaxflow(t, "backward-edge", g, ff)
}

// TestFlowEdgeMethods checks the FlowEdge helpers directly.
func TestFlowEdgeMethods(t *testing.T) {
	e := FlowEdge{From: 2, To: 5, Capacity: 10}
	if e.Other(2) != 5 || e.Other(5) != 2 {
		t.Errorf("Other: got %d/%d, expected 5/2", e.Other(2), e.Other(5))
	}
	if r := e.ResidualCapacityTo(5); r != 10 {
		t.Errorf("forward residual = %v, expected 10", r)
	}
	if r := e.ResidualCapacityTo(2); r != 0 {
		t.Errorf("backward residual = %v, expected 0", r)
	}
	e.AddResidualFlowTo(5, 4)
	if e.Flow != 4 {
		t.Errorf("Flow = %v, expected 4", e.Flow)
	}
	if r := e.ResidualCapacityTo(5); r != 6 {
		t.Errorf("forward residual = %v, expected 6", r)
	}
	if r := e.ResidualCapacityTo(2); r != 4 {
		t.Errorf("backward residual = %v, expected 4", r)
	}
	e.AddResidualFlowTo(2, 4) // cancel it all
	if e.Flow != 0 {
		t.Errorf("Flow = %v, expected 0", e.Flow)
	}
	self := FlowEdge{From: 3, To: 3, Capacity: 7}
	if self.Other(3) != 3 {
		t.Errorf("self-loop Other = %d, expected 3", self.Other(3))
	}
	expectPanic(t, "AddResidualFlowTo with negative delta", "AddResidualFlowTo", func() { e.AddResidualFlowTo(5, -1) })
	expectPanic(t, "AddResidualFlowTo with NaN delta", "AddResidualFlowTo", func() { e.AddResidualFlowTo(5, math.NaN()) })
	expectPanic(t, "AddResidualFlowTo overflowing the capacity", "drove the flow", func() { e.AddResidualFlowTo(5, 11) })
	expectPanic(t, "AddResidualFlowTo driving the flow negative", "drove the flow", func() { e.AddResidualFlowTo(2, 1) })
	if e.Flow != 0 {
		t.Errorf("Flow = %v after rejected updates, expected 0", e.Flow)
	}
}

// TestNetworkBasics checks the container surface: V/E/Adj, out-of-range
// reports, and the self-loop double-appearance convention.
func TestNetworkBasics(t *testing.T) {
	g := NewFlowNetwork(4)
	g.AddEdge(0, 1, 2)
	g.AddEdge(1, 2, 3)
	g.AddEdge(2, 2, 9) // self-loop
	if g.V() != 4 || g.E() != 3 {
		t.Errorf("V/E = %d/%d, expected 4/3", g.V(), g.E())
	}
	if len(g.Adj(0)) != 1 || len(g.Adj(1)) != 2 || len(g.Adj(2)) != 3 || len(g.Adj(3)) != 0 {
		t.Errorf("Adj lengths = %d/%d/%d/%d, expected 1/2/3/0 (each edge in both endpoints' lists, self-loop twice)",
			len(g.Adj(0)), len(g.Adj(1)), len(g.Adj(2)), len(g.Adj(3)))
	}
	if e := g.Adj(0)[0]; e.From != 0 || e.To != 1 || e.Capacity != 2 || e.Flow != 0 {
		t.Errorf("Adj(0)[0] = %+v, expected {0 1 2 0}", e)
	}
	if g.AddEdge(0, 4, 1) || g.AddEdge(-1, 2, 1) {
		t.Errorf("out-of-range AddEdge reported true")
	}
	if g.Adj(4) != nil || g.Adj(-1) != nil {
		t.Errorf("out-of-range Adj returned non-nil")
	}
	if g.E() != 3 {
		t.Errorf("E = %d after rejected AddEdge calls, expected 3", g.E())
	}
}

// -------------------------------------------------------------------------------------------------------
// Snapshot semantics
// -------------------------------------------------------------------------------------------------------

// TestSnapshotSemantics verifies that NewFordFulkerson never mutates the
// caller's network and that the result is immutable: edges added after
// construction are not reflected.
func TestSnapshotSemantics(t *testing.T) {
	g := NewFlowNetwork(3)
	g.AddEdge(0, 1, 2)
	g.AddEdge(1, 2, 2)
	ff := NewFordFulkerson(g, 0, 2)
	if ff.Value() != 2 {
		t.Fatalf("Value = %v, expected 2", ff.Value())
	}
	// The caller's network is untouched: every stored flow is still 0.
	for v := 0; v < g.V(); v++ {
		for _, e := range g.Adj(v) {
			if e.Flow != 0 {
				t.Errorf("network edge %d->%d has flow %v after NewFordFulkerson (network mutated)", e.From, e.To, e.Flow)
			}
		}
	}
	// Later AddEdge calls are not reflected in the result.
	g.AddEdge(0, 2, 100)
	if ff.Value() != 2 {
		t.Errorf("Value = %v after later AddEdge, expected 2 (snapshot semantics)", ff.Value())
	}
	// And Edges returns copies: mutating the slice must not corrupt the result.
	edges := ff.Edges()
	if len(edges) != 2 {
		t.Fatalf("len(Edges) = %d, expected 2", len(edges))
	}
	edges[0].Flow = -1000
	for _, e := range ff.Edges() {
		if e.Flow < 0 {
			t.Errorf("mutating the Edges slice corrupted the result: %+v", e)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Panic contract and nil tolerance
// -------------------------------------------------------------------------------------------------------

func TestMaxflowPanics(t *testing.T) {
	var nilNet *FlowNetwork
	zeroNet := &FlowNetwork{}

	expectPanic(t, "NewFlowNetwork(0)", "NewFlowNetwork", func() { NewFlowNetwork(0) })
	expectPanic(t, "NewFlowNetwork(-3)", "need v >= 1", func() { NewFlowNetwork(-3) })
	expectPanic(t, "AddEdge on a nil network", "AddEdge", func() { nilNet.AddEdge(0, 1, 1) })
	expectPanic(t, "AddEdge with negative capacity", "AddEdge", func() { NewFlowNetwork(2).AddEdge(0, 1, -1) })
	expectPanic(t, "AddEdge with NaN capacity", "negative or NaN", func() { NewFlowNetwork(2).AddEdge(0, 1, math.NaN()) })
	expectPanic(t, "NewFordFulkerson on a nil network", "NewFordFulkerson", func() { NewFordFulkerson(nilNet, 0, 1) })
	expectPanic(t, "NewFordFulkerson on a zero-value network", "nil or empty network", func() { NewFordFulkerson(zeroNet, 0, 1) })
	expectPanic(t, "NewFordFulkerson with out-of-range source", "out-of-range source", func() { NewFordFulkerson(NewFlowNetwork(3), 3, 1) })
	expectPanic(t, "NewFordFulkerson with negative source", "out-of-range source", func() { NewFordFulkerson(NewFlowNetwork(3), -1, 1) })
	expectPanic(t, "NewFordFulkerson with out-of-range sink", "out-of-range sink", func() { NewFordFulkerson(NewFlowNetwork(3), 0, -1) })
	expectPanic(t, "NewFordFulkerson with s == t", "source == sink", func() { NewFordFulkerson(NewFlowNetwork(3), 1, 1) })
}

// TestMaxflowNilTolerated verifies that nil networks and nil result
// objects answer every read with the empty result, and that the
// zero-value network reports false from AddEdge (the dijkstra
// precedent: every endpoint is out of range).
func TestMaxflowNilTolerated(t *testing.T) {
	var nilNet *FlowNetwork
	if nilNet.V() != 0 || nilNet.E() != 0 || nilNet.Adj(0) != nil {
		t.Errorf("Expected a nil network to report 0/0/nil.")
	}
	zeroNet := &FlowNetwork{}
	if zeroNet.V() != 0 || zeroNet.E() != 0 || zeroNet.Adj(0) != nil {
		t.Errorf("Expected a zero-value network to report 0/0/nil.")
	}
	if zeroNet.AddEdge(0, 0, 1) {
		t.Errorf("Expected AddEdge on a zero-value network to report false.")
	}

	var nilFF *FordFulkerson
	if nilFF.Value() != 0 || nilFF.InMinCut(0) || nilFF.S() != -1 || nilFF.T() != -1 || nilFF.Edges() != nil {
		t.Errorf("Expected a nil FordFulkerson to report 0/false/-1/-1/nil.")
	}

	g, _ := parseFlowNetwork(t, tinyFNData)
	ff := NewFordFulkerson(g, 0, 5)
	if ff.InMinCut(-1) || ff.InMinCut(6) {
		t.Errorf("Expected out-of-range InMinCut to report false.")
	}
	if ff.S() != 0 || ff.T() != 5 {
		t.Errorf("S/T = %d/%d, expected 0/5", ff.S(), ff.T())
	}
}

// -------------------------------------------------------------------------------------------------------
// Benchmarks
// -------------------------------------------------------------------------------------------------------

func BenchmarkFordFulkerson(b *testing.B) {
	g := benchmarkNetwork(2000, 8000, 42)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewFordFulkerson(g, 0, 1999)
	}
}
