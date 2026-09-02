package mst_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"strconv"
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
// (the tinyEWG weights are not exactly representable in float64).
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// tinyEWGData is Sedgwick's classic tinyEWG.txt dataset (§4.3),
// embedded verbatim from
// https://algs4.cs.princeton.edu/43mst/tinyEWG.txt — 8 vertices, 16
// edges, all weights distinct (so the MST is unique).
const tinyEWGData = `8
16
4 5 0.35
4 7 0.37
5 7 0.28
0 7 0.16
1 5 0.32
0 4 0.38
2 3 0.17
1 7 0.19
0 2 0.26
1 2 0.36
1 3 0.29
2 7 0.34
6 2 0.40
3 6 0.52
6 0 0.58
6 4 0.93
`

// parseEdgeWeightedGraph parses the algs4 edge-weighted graph format:
// V, E, then E lines of "v w weight".
func parseEdgeWeightedGraph(t *testing.T, data string) *dijkstra_ts.EdgeWeightedGraph {
	t.Helper()
	fields := strings.Fields(data)
	if len(fields) < 2 {
		t.Fatalf("bad graph data: %q", data)
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
	g := dijkstra_ts.NewEdgeWeightedGraph(n)
	for i := 0; i < m; i++ {
		v, err1 := strconv.Atoi(fields[2+3*i])
		w, err2 := strconv.Atoi(fields[3+3*i])
		wt, err3 := strconv.ParseFloat(fields[4+3*i], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatalf("bad edge %d: %v %v %v", i, err1, err2, err3)
		}
		if !g.AddEdge(dijkstra_ts.Edge{V: v, W: w, Weight: wt}) {
			t.Fatalf("edge %d (%d--%d) rejected", i, v, w)
		}
	}
	return g
}

// edgePair is an unordered pair of vertices with the edge's weight,
// for comparing edge sets independent of order and orientation.
type edgePair struct {
	v, w   int
	weight float64
}

// edgeSet normalizes edges into a set of unordered pairs.
func edgeSet(edges []dijkstra_ts.Edge) map[edgePair]bool {
	set := make(map[edgePair]bool, len(edges))
	for _, e := range edges {
		v, w := e.V, e.W
		if v > w {
			v, w = w, v
		}
		set[edgePair{v, w, e.Weight}] = true
	}
	return set
}

// checkForest verifies the spanning-forest property of a result: the
// edges contain no cycle, every edge joins vertices that are connected
// in the graph, and Len == V - (number of connected components of g).
// The connectivity bookkeeping is done with plain slices — deliberately
// not pluto/union_find, which Kruskal itself uses.
func checkForest(t *testing.T, name string, n int, g *dijkstra_ts.EdgeWeightedGraph, edges []dijkstra_ts.Edge) {
	t.Helper()

	// Components of the graph, by plain BFS over its adjacency.
	comp := make([]int, n)
	for i := range comp {
		comp[i] = -1
	}
	nComp := 0
	for s := 0; s < n; s++ {
		if comp[s] != -1 {
			continue
		}
		stack := []int{s}
		comp[s] = nComp
		for len(stack) > 0 {
			v := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, e := range g.Adj(v) {
				if w := e.Other(v); comp[w] == -1 {
					comp[w] = nComp
					stack = append(stack, w)
				}
			}
		}
		nComp++
	}

	if len(edges) != n-nComp {
		t.Errorf("%s: Len = %d, expected %d (V=%d, %d components)", name, len(edges), n-nComp, n, nComp)
	}

	// No cycle, and every edge stays inside one component: a tiny
	// in-test union-find over the result edges.
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(p int) int {
		for parent[p] != p {
			parent[p] = parent[parent[p]]
			p = parent[p]
		}
		return p
	}
	for _, e := range edges {
		if comp[e.V] != comp[e.W] {
			t.Errorf("%s: edge %v joins two components of the graph", name, e)
		}
		rv, rw := find(e.V), find(e.W)
		if rv == rw {
			t.Errorf("%s: edge %v closes a cycle in the result", name, e)
		}
		parent[rw] = rv
	}
}

// -------------------------------------------------------------------------------------------------------
// Known answers on tinyEWG
// -------------------------------------------------------------------------------------------------------

// knownTinyEWGMST is the unique MST of tinyEWG (all 16 weights are
// distinct) — the same edge set the algs4 Java clients print, total
// weight 1.81 (https://algs4.cs.princeton.edu/43mst/LazyPrimMST.java.html).
var knownTinyEWGMST = []edgePair{
	{0, 7, 0.16},
	{1, 7, 0.19},
	{0, 2, 0.26},
	{2, 3, 0.17},
	{5, 7, 0.28},
	{4, 5, 0.35},
	{2, 6, 0.40},
}

// checkTinyEWG verifies a result against the known tinyEWG answer.
func checkTinyEWG(t *testing.T, name string, g *dijkstra_ts.EdgeWeightedGraph, edges []dijkstra_ts.Edge, weight float64, length int) {
	t.Helper()
	if length != 7 {
		t.Errorf("%s: Len = %d, expected 7 (V-1 on a connected graph)", name, length)
	}
	if !almostEqual(weight, 1.81) {
		t.Errorf("%s: Weight = %v, expected 1.81", name, weight)
	}
	sum := 0.0
	for _, e := range edges {
		sum += e.Weight
	}
	if !almostEqual(sum, weight) {
		t.Errorf("%s: Edges sum to %v, Weight reports %v", name, sum, weight)
	}
	set := edgeSet(edges)
	if len(set) != len(edges) {
		t.Errorf("%s: Edges contains duplicates: %v", name, edges)
	}
	for _, want := range knownTinyEWGMST {
		if !set[want] {
			t.Errorf("%s: missing known MST edge %d--%d (%g); got %v", name, want.v, want.w, want.weight, edges)
		}
	}
	checkForest(t, name, 8, g, edges)
}

func TestTinyEWGLazyPrim(t *testing.T) {
	g := parseEdgeWeightedGraph(t, tinyEWGData)
	mst := NewLazyPrimMST(g)
	checkTinyEWG(t, "LazyPrimMST", g, mst.Edges(), mst.Weight(), mst.Len())
}

func TestTinyEWGPrim(t *testing.T) {
	g := parseEdgeWeightedGraph(t, tinyEWGData)
	mst := NewPrimMST(g)
	checkTinyEWG(t, "PrimMST", g, mst.Edges(), mst.Weight(), mst.Len())
}

func TestTinyEWGKruskal(t *testing.T) {
	g := parseEdgeWeightedGraph(t, tinyEWGData)
	mst := NewKruskalMST(g)
	checkTinyEWG(t, "KruskalMST", g, mst.Edges(), mst.Weight(), mst.Len())
}

// TestTinyEWGAgreement checks that all three algorithms agree on the
// total weight — and, since tinyEWG's MST is unique, on the edge set.
func TestTinyEWGAgreement(t *testing.T) {
	g := parseEdgeWeightedGraph(t, tinyEWGData)
	lp := NewLazyPrimMST(g)
	pr := NewPrimMST(g)
	kr := NewKruskalMST(g)
	if !almostEqual(lp.Weight(), pr.Weight()) || !almostEqual(pr.Weight(), kr.Weight()) {
		t.Errorf("Weights disagree: lazy %v, eager %v, kruskal %v", lp.Weight(), pr.Weight(), kr.Weight())
	}
	for p := range edgeSet(lp.Edges()) {
		if !edgeSet(pr.Edges())[p] || !edgeSet(kr.Edges())[p] {
			t.Errorf("Edge %v not produced by all three algorithms.", p)
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Forests, self-loops, parallel edges, negative weights
// -------------------------------------------------------------------------------------------------------

// TestDisconnectedForest checks that a disconnected graph yields the
// minimum spanning FOREST: V - (components) edges, one tree per
// component.
func TestDisconnectedForest(t *testing.T) {
	// Components {0,1,2} and {3,4}; vertex 5 isolated.
	g := dijkstra_ts.NewEdgeWeightedGraph(6)
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 1, Weight: 1})
	g.AddEdge(dijkstra_ts.Edge{V: 1, W: 2, Weight: 2})
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 2, Weight: 9}) // cycle edge, must be skipped
	g.AddEdge(dijkstra_ts.Edge{V: 3, W: 4, Weight: 3})

	type result struct {
		name   string
		weight float64
		length int
		edges  []dijkstra_ts.Edge
	}
	lp, pr, kr := NewLazyPrimMST(g), NewPrimMST(g), NewKruskalMST(g)
	for _, r := range []result{
		{"LazyPrimMST", lp.Weight(), lp.Len(), lp.Edges()},
		{"PrimMST", pr.Weight(), pr.Len(), pr.Edges()},
		{"KruskalMST", kr.Weight(), kr.Len(), kr.Edges()},
	} {
		if r.length != 3 { // 6 vertices, 3 components
			t.Errorf("%s: Len = %d, expected 3 (V - components)", r.name, r.length)
		}
		if r.weight != 6 { // 1 + 2 + 3
			t.Errorf("%s: Weight = %v, expected 6", r.name, r.weight)
		}
		checkForest(t, r.name, 6, g, r.edges)
	}
}

// TestSelfLoopAndParallelEdges checks that a self-loop never joins the
// tree and that the cheapest of parallel edges wins.
func TestSelfLoopAndParallelEdges(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedGraph(3)
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 0, Weight: -5}) // even a cheap self-loop is never a tree edge
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 1, Weight: 9})
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 1, Weight: 4}) // parallel, cheaper
	g.AddEdge(dijkstra_ts.Edge{V: 1, W: 2, Weight: 3})

	lp, pr, kr := NewLazyPrimMST(g), NewPrimMST(g), NewKruskalMST(g)
	for name, edges := range map[string][]dijkstra_ts.Edge{
		"LazyPrimMST": lp.Edges(),
		"PrimMST":     pr.Edges(),
		"KruskalMST":  kr.Edges(),
	} {
		if len(edges) != 2 {
			t.Errorf("%s: Len = %d, expected 2", name, len(edges))
		}
		set := edgeSet(edges)
		if !set[edgePair{0, 1, 4}] || !set[edgePair{1, 2, 3}] {
			t.Errorf("%s: edges = %v, expected 0--1(4) and 1--2(3)", name, edges)
		}
	}
	if lp.Weight() != 7 || pr.Weight() != 7 || kr.Weight() != 7 {
		t.Errorf("Weights = %v/%v/%v, expected 7", lp.Weight(), pr.Weight(), kr.Weight())
	}
}

// TestNegativeWeights checks that negative edge weights are accepted
// and handled correctly — an MST is well-defined for arbitrary weights.
func TestNegativeWeights(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedGraph(4)
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 1, Weight: 1})
	g.AddEdge(dijkstra_ts.Edge{V: 1, W: 2, Weight: -5})
	g.AddEdge(dijkstra_ts.Edge{V: 2, W: 3, Weight: 2})
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 3, Weight: 4})

	// MST: 1--2(-5), 0--1(1), 2--3(2) = -2.
	want := []edgePair{{1, 2, -5}, {0, 1, 1}, {2, 3, 2}}
	lp, pr, kr := NewLazyPrimMST(g), NewPrimMST(g), NewKruskalMST(g)
	if lp.Weight() != -2 || pr.Weight() != -2 || kr.Weight() != -2 {
		t.Errorf("Weights = %v/%v/%v, expected -2", lp.Weight(), pr.Weight(), kr.Weight())
	}
	for name, edges := range map[string][]dijkstra_ts.Edge{
		"LazyPrimMST": lp.Edges(),
		"PrimMST":     pr.Edges(),
		"KruskalMST":  kr.Edges(),
	} {
		set := edgeSet(edges)
		if len(edges) != 3 {
			t.Errorf("%s: Len = %d, expected 3", name, len(edges))
		}
		for _, w := range want {
			if !set[w] {
				t.Errorf("%s: missing edge %v; got %v", name, w, edges)
			}
		}
	}
}

// TestSingleVertex checks the smallest legal graph: one vertex, no
// edges, an empty tree of weight 0.
func TestSingleVertex(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedGraph(1)
	lp, pr, kr := NewLazyPrimMST(g), NewPrimMST(g), NewKruskalMST(g)
	for name, m := range map[string]struct {
		w float64
		l int
	}{"LazyPrimMST": {lp.Weight(), lp.Len()}, "PrimMST": {pr.Weight(), pr.Len()}, "KruskalMST": {kr.Weight(), kr.Len()}} {
		if m.w != 0 || m.l != 0 {
			t.Errorf("%s: Weight/Len = %v/%d, expected 0/0", name, m.w, m.l)
		}
	}
}

// TestImmutableResult verifies that a query object is a snapshot: edges
// added after construction are not reflected.
func TestImmutableResult(t *testing.T) {
	g := dijkstra_ts.NewEdgeWeightedGraph(3)
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 1, Weight: 10})
	g.AddEdge(dijkstra_ts.Edge{V: 1, W: 2, Weight: 10})
	kr := NewKruskalMST(g)
	g.AddEdge(dijkstra_ts.Edge{V: 0, W: 2, Weight: 1}) // a much cheaper route
	if kr.Weight() != 20 || kr.Len() != 2 {
		t.Errorf("Weight/Len = %v/%d, expected 20/2 (snapshot semantics)", kr.Weight(), kr.Len())
	}
}

// -------------------------------------------------------------------------------------------------------
// Panic contract and nil tolerance
// -------------------------------------------------------------------------------------------------------

func TestMSTPanics(t *testing.T) {
	var nilGraph *dijkstra_ts.EdgeWeightedGraph
	zeroGraph := &dijkstra_ts.EdgeWeightedGraph{}

	expectPanic(t, "NewLazyPrimMST on a nil graph", "mst_ts: NewLazyPrimMST", func() { NewLazyPrimMST(nilGraph) })
	expectPanic(t, "NewLazyPrimMST on a nil graph", "nil or empty graph", func() { NewLazyPrimMST(nilGraph) })
	expectPanic(t, "NewLazyPrimMST on a zero-value graph", "NewLazyPrimMST", func() { NewLazyPrimMST(zeroGraph) })
	expectPanic(t, "NewPrimMST on a nil graph", "mst_ts: NewPrimMST", func() { NewPrimMST(nilGraph) })
	expectPanic(t, "NewPrimMST on a zero-value graph", "nil or empty graph", func() { NewPrimMST(zeroGraph) })
	expectPanic(t, "NewKruskalMST on a nil graph", "mst_ts: NewKruskalMST", func() { NewKruskalMST(nilGraph) })
	expectPanic(t, "NewKruskalMST on a zero-value graph", "NewKruskalMST", func() { NewKruskalMST(zeroGraph) })
}

// TestMSTNilTolerated verifies that nil result objects answer every
// read with the empty result.
func TestMSTNilTolerated(t *testing.T) {
	var nilLP *LazyPrimMST
	if nilLP.Edges() != nil || nilLP.Weight() != 0 || nilLP.Len() != 0 {
		t.Errorf("Expected a nil LazyPrimMST to report nil/0/0.")
	}
	var nilPR *PrimMST
	if nilPR.Edges() != nil || nilPR.Weight() != 0 || nilPR.Len() != 0 {
		t.Errorf("Expected a nil PrimMST to report nil/0/0.")
	}
	var nilKR *KruskalMST
	if nilKR.Edges() != nil || nilKR.Weight() != 0 || nilKR.Len() != 0 {
		t.Errorf("Expected a nil KruskalMST to report nil/0/0.")
	}
}
