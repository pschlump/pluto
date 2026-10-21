package global_mincut

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/maxflow"
)

// expectPanic runs fn and requires it to panic with a message
// containing want.
func expectPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected a panic containing %q, got none", want)
			return
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, want) {
			t.Errorf("Expected panic containing %q, got %q", want, msg)
		}
	}()
	fn()
}

// buildGraph builds an EdgeWeightedGraph from an edge list, failing the
// test on out-of-range endpoints (test inputs are always in range).
func buildGraph(t *testing.T, n int, edges []dijkstra.Edge) *dijkstra.EdgeWeightedGraph {
	t.Helper()
	g := dijkstra.NewEdgeWeightedGraph(n)
	for _, e := range edges {
		if !g.AddEdge(e) {
			t.Fatalf("Test bug: edge %d--%d out of range for n=%d", e.V, e.W, n)
		}
	}
	return g
}

// bruteForceMinCut enumerates every bipartition (subsets containing 0,
// complement nonempty) and returns the minimum crossing weight — the
// exact oracle for small graphs.  mask's bit v-1 is vertex v's side; the
// all-ones mask (empty complement) is excluded, and n must be >= 2.
func bruteForceMinCut(t *testing.T, g *dijkstra.EdgeWeightedGraph) float64 {
	t.Helper()
	n := g.V()
	best := math.Inf(1)
	for mask := 0; mask < 1<<(n-1)-1; mask++ {
		inS := make([]bool, n)
		inS[0] = true
		for v := 1; v < n; v++ {
			inS[v] = mask&(1<<(v-1)) != 0
		}
		var w float64
		for v := 0; v < n; v++ {
			for _, e := range g.Adj(v) {
				if u := e.Other(v); u > v && inS[v] != inS[u] {
					w += e.Weight
				}
			}
		}
		if w < best {
			best = w
		}
	}
	return best
}

// verifyCertificate checks that c's cut is a real nonempty proper
// bipartition side of g whose crossing weight is exactly c.Weight().
func verifyCertificate(t *testing.T, g *dijkstra.EdgeWeightedGraph, c *GlobalMincut) {
	t.Helper()
	n := g.V()
	cut := c.Cut()
	if len(cut) == 0 || len(cut) >= n {
		t.Fatalf("Cut %v is not a nonempty proper side of %d vertices", cut, n)
	}
	for _, v := range cut {
		if !c.InCut(v) {
			t.Errorf("Cut contains %d but InCut(%d) is false", v, v)
		}
	}
	var w float64
	for v := 0; v < n; v++ {
		for _, e := range g.Adj(v) {
			if u := e.Other(v); u > v && c.InCut(v) != c.InCut(u) {
				w += e.Weight
			}
		}
	}
	if w != c.Weight() {
		t.Errorf("Crossing weight of the returned cut is %g, reported Weight is %g", w, c.Weight())
	}
}

// maxflowMinCut computes the global min cut by the n−1-maxflows route:
// fix source 0, take the minimum over every other sink of the max-flow
// value (each undirected edge becomes two directed edges of the same
// capacity — the standard reduction).  Independent of Stoer–Wagner.
func maxflowMinCut(t *testing.T, g *dijkstra.EdgeWeightedGraph) float64 {
	t.Helper()
	n := g.V()
	best := math.Inf(1)
	for t0 := 1; t0 < n; t0++ {
		net := maxflow.NewFlowNetwork(n)
		for v := 0; v < n; v++ {
			for _, e := range g.Adj(v) {
				if u := e.Other(v); u != v && u > v {
					net.AddEdge(v, u, e.Weight)
					net.AddEdge(u, v, e.Weight)
				}
			}
		}
		if val := maxflow.NewFordFulkerson(net, 0, t0).Value(); val < best {
			best = val
		}
	}
	return best
}

func TestGlobalMincutTwoTriangles(t *testing.T) {
	// Two triangles of weight-10 edges joined by a weight-1 bridge: the
	// min cut severs just the bridge.
	g := buildGraph(t, 6, []dijkstra.Edge{
		{V: 0, W: 1, Weight: 10}, {V: 1, W: 2, Weight: 10}, {V: 2, W: 0, Weight: 10},
		{V: 3, W: 4, Weight: 10}, {V: 4, W: 5, Weight: 10}, {V: 5, W: 3, Weight: 10},
		{V: 2, W: 3, Weight: 1},
	})
	c := NewGlobalMincut(g)
	if c.Weight() != 1 {
		t.Errorf("Expected min cut weight 1, got %g", c.Weight())
	}
	if cut := c.Cut(); len(cut) != 3 {
		t.Errorf("Expected a 3-vertex cut side (one whole triangle), got %v", cut)
	}
	verifyCertificate(t, g, c)
}

func TestGlobalMincutChain(t *testing.T) {
	g := buildGraph(t, 3, []dijkstra.Edge{
		{V: 0, W: 1, Weight: 1}, {V: 1, W: 2, Weight: 2},
	})
	c := NewGlobalMincut(g)
	if c.Weight() != 1 {
		t.Errorf("Expected min cut weight 1, got %g", c.Weight())
	}
	verifyCertificate(t, g, c)
}

func TestGlobalMincutDisconnected(t *testing.T) {
	// A disconnected graph has a min cut of weight 0: the reported side
	// is a whole component.
	g := buildGraph(t, 4, []dijkstra.Edge{
		{V: 0, W: 1, Weight: 5}, {V: 2, W: 3, Weight: 3},
	})
	c := NewGlobalMincut(g)
	if c.Weight() != 0 {
		t.Errorf("Expected min cut weight 0 on a disconnected graph, got %g", c.Weight())
	}
	verifyCertificate(t, g, c)
}

func TestGlobalMincutParallelEdgesSum(t *testing.T) {
	// Two parallel 0-1 edges (total capacity 4) vs a single weight-3
	// edge: the min cut is 3.
	g := buildGraph(t, 3, []dijkstra.Edge{
		{V: 0, W: 1, Weight: 2}, {V: 0, W: 1, Weight: 2}, {V: 1, W: 2, Weight: 3},
	})
	c := NewGlobalMincut(g)
	if c.Weight() != 3 {
		t.Errorf("Expected min cut weight 3 with parallel edges summed, got %g", c.Weight())
	}
	verifyCertificate(t, g, c)
}

func TestGlobalMincutSelfLoopsIgnored(t *testing.T) {
	// A self-loop never crosses a cut: the triangle's min cut (10, one
	// vertex against the other two) is unchanged by a weight-100 loop.
	g := buildGraph(t, 3, []dijkstra.Edge{
		{V: 0, W: 1, Weight: 5}, {V: 1, W: 2, Weight: 5}, {V: 2, W: 0, Weight: 5},
		{V: 0, W: 0, Weight: 100},
	})
	c := NewGlobalMincut(g)
	if c.Weight() != 10 {
		t.Errorf("Expected min cut weight 10 with the self-loop ignored, got %g", c.Weight())
	}
	verifyCertificate(t, g, c)
}

// TestGlobalMincutSingleVertex: one vertex has no bipartition — the
// zero result, but no panic (a self-loop-only graph is still legal).
func TestGlobalMincutSingleVertex(t *testing.T) {
	g := buildGraph(t, 1, []dijkstra.Edge{{V: 0, W: 0, Weight: 7}})
	c := NewGlobalMincut(g)
	if c.Weight() != 0 {
		t.Errorf("Expected Weight 0 for a single-vertex graph, got %g", c.Weight())
	}
	if cut := c.Cut(); cut != nil {
		t.Errorf("Expected Cut nil for a single-vertex graph, got %v", cut)
	}
	if c.InCut(0) {
		t.Errorf("Expected InCut(0) false for a single-vertex graph")
	}
}

func TestGlobalMincutPanics(t *testing.T) {
	var nilGraph *dijkstra.EdgeWeightedGraph
	expectPanic(t, "nil or empty graph", func() { NewGlobalMincut(nilGraph) })
	var zeroGraph dijkstra.EdgeWeightedGraph
	expectPanic(t, "nil or empty graph", func() { NewGlobalMincut(&zeroGraph) })
	expectPanic(t, "edge 3--0 has negative weight", func() {
		NewGlobalMincut(buildGraph(t, 4, []dijkstra.Edge{{V: 3, W: 0, Weight: -1}}))
	})
	expectPanic(t, "non-finite weight", func() {
		NewGlobalMincut(buildGraph(t, 2, []dijkstra.Edge{{V: 0, W: 1, Weight: math.NaN()}}))
	})
	expectPanic(t, "non-finite weight", func() {
		NewGlobalMincut(buildGraph(t, 2, []dijkstra.Edge{{V: 0, W: 1, Weight: math.Inf(1)}}))
	})
}

func TestGlobalMincutNilTolerated(t *testing.T) {
	var c *GlobalMincut
	if c.Weight() != 0 {
		t.Errorf("Expected Weight 0 on a nil receiver")
	}
	if cut := c.Cut(); cut != nil {
		t.Errorf("Expected Cut nil on a nil receiver, got %v", cut)
	}
	if c.InCut(0) {
		t.Errorf("Expected InCut false on a nil receiver")
	}
}

// TestGlobalMincutRandomizedBruteForce cross-checks every result
// against the all-bipartitions oracle (integer weights — exact float
// comparison) and verifies the cut certificate.
func TestGlobalMincutRandomizedBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	for step := 0; step < 400; step++ {
		n := 2 + rng.Intn(10)
		g := dijkstra.NewEdgeWeightedGraph(n)
		m := rng.Intn(3 * n)
		for i := 0; i < m; i++ {
			// ~10% self-loops, otherwise random edges; weights 0..20.
			v := rng.Intn(n)
			w := v
			if rng.Intn(10) > 0 {
				w = rng.Intn(n)
			}
			g.AddEdge(dijkstra.Edge{V: v, W: w, Weight: float64(rng.Intn(21))})
		}
		c := NewGlobalMincut(g)
		if want := bruteForceMinCut(t, g); c.Weight() != want {
			t.Fatalf("step %d: Weight %g, brute force %g (n=%d)", step, c.Weight(), want, n)
		}
		verifyCertificate(t, g, c)
	}
}

// TestGlobalMincutMaxflowAgreement cross-checks Stoer–Wagner against
// the n−1-maxflows route on medium graphs — two independent algorithm
// families must agree exactly (integer capacities).
func TestGlobalMincutMaxflowAgreement(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	for step := 0; step < 12; step++ {
		n := 30
		g := dijkstra.NewEdgeWeightedGraph(n)
		for i := 0; i < 90; i++ {
			g.AddEdge(dijkstra.Edge{V: rng.Intn(n), W: rng.Intn(n), Weight: float64(1 + rng.Intn(10))})
		}
		if got, want := NewGlobalMincut(g).Weight(), maxflowMinCut(t, g); got != want {
			t.Fatalf("step %d: Stoer–Wagner %g, max-flow route %g", step, got, want)
		}
	}
}
