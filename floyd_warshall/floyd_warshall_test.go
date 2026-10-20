package floyd_warshall

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/pschlump/pluto/bellman_ford"
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

// checkPath verifies that fw.Path(v, w) is a connected v -> w walk whose
// weights sum to fw.Dist(v, w), with every edge present in g.
func checkPath(t *testing.T, name string, fw *FloydWarshall, g *dijkstra.EdgeWeightedDigraph, v, w int) {
	t.Helper()
	path := fw.Path(v, w)
	if path == nil {
		t.Fatalf("%s: Path(%d, %d) = nil for a reachable pair", name, v, w)
	}
	if len(path) == 0 {
		if v != w {
			t.Fatalf("%s: Path(%d, %d) is empty but %d != %d", name, v, w, v, w)
		}
		return
	}
	if path[0].From != v || path[len(path)-1].To != w {
		t.Fatalf("%s: Path(%d, %d) has wrong endpoints: %v", name, v, w, path)
	}
	members := map[dijkstra.DirectedEdge]int{}
	for x := range g.V() {
		for _, e := range g.Adj(x) {
			members[e]++
		}
	}
	sum := 0.0
	for i, e := range path {
		if i > 0 && path[i-1].To != e.From {
			t.Fatalf("%s: Path(%d, %d) is not connected at edge %d", name, v, w, i)
		}
		if members[e] == 0 {
			t.Fatalf("%s: Path(%d, %d) uses edge %v which is not in the graph", name, v, w, e)
		}
		members[e]--
		sum += e.Weight
	}
	if !almostEqual(sum, fw.Dist(v, w)) {
		t.Fatalf("%s: Path(%d, %d) sums to %v, Dist = %v", name, v, w, sum, fw.Dist(v, w))
	}
}

// checkCycle verifies that cycle is a genuine negative cycle of g: a
// closed connected walk with every edge in g and a negative weight sum.
func checkCycle(t *testing.T, name string, g *dijkstra.EdgeWeightedDigraph, cycle []dijkstra.DirectedEdge) {
	t.Helper()
	if len(cycle) == 0 {
		t.Fatalf("%s: NegativeCycle() returned no edges", name)
	}
	members := map[dijkstra.DirectedEdge]int{}
	for v := range g.V() {
		for _, e := range g.Adj(v) {
			members[e]++
		}
	}
	sum := 0.0
	for i, e := range cycle {
		if next := cycle[(i+1)%len(cycle)]; e.To != next.From {
			t.Fatalf("%s: NegativeCycle() is not a cycle at edge %d", name, i)
		}
		if members[e] == 0 {
			t.Fatalf("%s: NegativeCycle() uses edge %v which is not in the graph", name, e)
		}
		members[e]--
		sum += e.Weight
	}
	if sum >= 0 {
		t.Fatalf("%s: NegativeCycle() weights sum to %v, expected negative", name, sum)
	}
}

// TestTinyEWDAgainstDijkstra cross-checks the whole 8x8 answer matrix
// against dijkstra's all-pairs query object (tinyEWD is non-negative, so
// Dijkstra is exact) — two different algorithms over the same graph.
func TestTinyEWDAgainstDijkstra(t *testing.T) {
	g := parseDigraph(t, tinyEWDData)
	fw := NewFloydWarshall(g)
	ap := dijkstra.NewDijkstraAllPairsSP(g)
	if fw.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = true on tinyEWD")
	}
	for v := range g.V() {
		for w := range g.V() {
			if got, want := fw.Dist(v, w), ap.Dist(v, w); !almostEqual(got, want) {
				t.Fatalf("Dist(%d, %d) = %v, dijkstra says %v", v, w, got, want)
			}
			if fw.HasPath(v, w) != ap.HasPath(v, w) {
				t.Fatalf("HasPath(%d, %d) disagrees with dijkstra", v, w)
			}
			if fw.HasPath(v, w) {
				checkPath(t, "tinyEWD", fw, g, v, w)
			} else if fw.Path(v, w) != nil {
				t.Fatalf("Path(%d, %d) = %v for an unreachable pair", v, w, fw.Path(v, w))
			}
		}
		if fw.Dist(v, v) != 0 {
			t.Fatalf("Dist(%d, %d) = %v, want 0", v, v, fw.Dist(v, v))
		}
	}
}

// TestTinyEWDnAgainstBellmanFord cross-checks the negative-weight
// dataset (no negative cycle) against bellman_ford run from every
// source — again two structurally different algorithms.
func TestTinyEWDnAgainstBellmanFord(t *testing.T) {
	g := parseDigraph(t, tinyEWDnData)
	fw := NewFloydWarshall(g)
	if fw.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = true on tinyEWDn (it has none)")
	}
	for s := range g.V() {
		sp := bellman_ford.NewBellmanFordSP(g, s)
		if sp.HasNegativeCycle() {
			t.Fatalf("bellman_ford found a negative cycle on tinyEWDn from %d", s)
		}
		for w := range g.V() {
			if got, want := fw.Dist(s, w), sp.DistTo(w); !almostEqual(got, want) {
				t.Fatalf("Dist(%d, %d) = %v, bellman_ford says %v", s, w, got, want)
			}
		}
	}
	// The negated edges make some routes through vertex 6 the cheap
	// ones: pin two known distances from the dataset.  The shortest
	// 0 -> 4 route is the long way round 0->2->7->3->6->4.
	if got, want := fw.Dist(6, 0), -1.40; !almostEqual(got, want) {
		t.Fatalf("Dist(6, 0) = %v, want %v", got, want)
	}
	if got, want := fw.Dist(0, 4), 0.26+0.34+0.39+0.52-1.25; !almostEqual(got, want) {
		t.Fatalf("Dist(0, 4) = %v, want %v (the route through 6)", got, want)
	}
	checkPath(t, "tinyEWDn", fw, g, 0, 4)
}

// TestNegativeCycles covers the detection surface: a table of graphs
// that DO have a negative cycle (in several shapes, including one no
// source-rooted search could see) and one that does not.
func TestNegativeCycles(t *testing.T) {
	newCase := func(edges ...dijkstra.DirectedEdge) *dijkstra.EdgeWeightedDigraph {
		n := 0
		for _, e := range edges {
			n = max(n, e.From+1, e.To+1)
		}
		g := dijkstra.NewEdgeWeightedDigraph(n)
		for _, e := range edges {
			g.AddEdge(e)
		}
		return g
	}
	positive := map[string]*dijkstra.EdgeWeightedDigraph{
		"zero-weight-cycle-is-not-negative": newCase(
			dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1},
			dijkstra.DirectedEdge{From: 1, To: 0, Weight: -1},
		),
	}
	negative := map[string]*dijkstra.EdgeWeightedDigraph{
		"negative-self-loop": newCase(
			dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1},
			dijkstra.DirectedEdge{From: 1, To: 1, Weight: -0.5},
		),
		"two-cycle": newCase(
			dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1},
			dijkstra.DirectedEdge{From: 1, To: 0, Weight: -2},
		),
		"three-cycle-away-from-zero": newCase(
			dijkstra.DirectedEdge{From: 0, To: 1, Weight: 5},
			dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1},
			dijkstra.DirectedEdge{From: 3, To: 4, Weight: -3},
			dijkstra.DirectedEdge{From: 4, To: 2, Weight: 1},
		),
		// The negative cycle sits in a component no source in the other
		// component can reach — bellman_ford from any vertex of the
		// other component would NOT find it; Floyd-Warshall must.
		"disconnected-component": newCase(
			dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2},
			dijkstra.DirectedEdge{From: 1, To: 0, Weight: 3},
			dijkstra.DirectedEdge{From: 2, To: 3, Weight: -1},
			dijkstra.DirectedEdge{From: 3, To: 4, Weight: 1},
			dijkstra.DirectedEdge{From: 4, To: 2, Weight: -1},
		),
		"cycle-plus-negative-noncycle-edges": newCase(
			dijkstra.DirectedEdge{From: 0, To: 1, Weight: -5},
			dijkstra.DirectedEdge{From: 1, To: 2, Weight: 2},
			dijkstra.DirectedEdge{From: 2, To: 3, Weight: -5},
			dijkstra.DirectedEdge{From: 3, To: 1, Weight: 2},
			dijkstra.DirectedEdge{From: 3, To: 4, Weight: -7},
		),
	}

	for name, g := range positive {
		fw := NewFloydWarshall(g)
		if fw.HasNegativeCycle() {
			t.Errorf("%s: HasNegativeCycle() = true, want false", name)
		}
		if c := fw.NegativeCycle(); c != nil {
			t.Errorf("%s: NegativeCycle() = %v, want nil", name, c)
		}
		for v := range g.V() {
			if fw.Dist(v, v) != 0 {
				t.Errorf("%s: Dist(%d, %d) = %v, want 0", name, v, v, fw.Dist(v, v))
			}
		}
	}
	for name, g := range negative {
		fw := NewFloydWarshall(g)
		if !fw.HasNegativeCycle() {
			t.Errorf("%s: HasNegativeCycle() = false, want true", name)
			continue
		}
		checkCycle(t, name, g, fw.NegativeCycle())
		// Two independent calls return equal, fresh slices.
		if a, b := fw.NegativeCycle(), fw.NegativeCycle(); &a[0] == &b[0] {
			t.Errorf("%s: NegativeCycle() returned the same backing array twice", name)
		}
		// The shortest-path questions are undefined then: every one of
		// them panics naming the negative cycle.
		for _, q := range []struct {
			name string
			fx   func()
		}{
			{"Dist", func() { fw.Dist(0, g.V()-1) }},
			{"HasPath", func() { fw.HasPath(0, g.V()-1) }},
			{"Path", func() { fw.Path(0, g.V()-1) }},
		} {
			expectPanic(t, name+"/"+q.name, "negative cycle", q.fx)
		}
	}
}

// TestParallelEdgesAndSelfLoops pins the matrix-initialization policy:
// parallels collapse to the minimum, non-negative self-loops are
// ignored, a negative self-loop is an immediate negative cycle.
func TestParallelEdgesAndSelfLoops(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 5})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2}) // the minimum wins
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 9})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 0, Weight: 7}) // positive self-loop: ignored
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 2, Weight: 0}) // zero self-loop: ignored

	fw := NewFloydWarshall(g)
	if got, want := fw.Dist(0, 1), 2.0; got != want {
		t.Fatalf("Dist(0, 1) = %v, want %v (the minimum parallel edge)", got, want)
	}
	if path := fw.Path(0, 1); len(path) != 1 || path[0].Weight != 2 {
		t.Fatalf("Path(0, 1) = %v, want the single weight-2 edge", path)
	}
	if got, want := fw.Dist(0, 2), 3.0; got != want {
		t.Fatalf("Dist(0, 2) = %v, want %v", got, want)
	}
	for v := range g.V() {
		if fw.Dist(v, v) != 0 {
			t.Fatalf("Dist(%d, %d) = %v, want 0 (self-loops ignored)", v, v, fw.Dist(v, v))
		}
		if p := fw.Path(v, v); p == nil || len(p) != 0 {
			t.Fatalf("Path(%d, %d) = %v, want an empty non-nil slice", v, v, p)
		}
	}

	// A negative self-loop is a negative cycle of length one.
	g2 := dijkstra.NewEdgeWeightedDigraph(2)
	g2.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	g2.AddEdge(dijkstra.DirectedEdge{From: 1, To: 1, Weight: -1})
	fw2 := NewFloydWarshall(g2)
	if !fw2.HasNegativeCycle() {
		t.Fatalf("HasNegativeCycle() = false with a negative self-loop")
	}
	c := fw2.NegativeCycle()
	if len(c) != 1 || c[0].From != 1 || c[0].To != 1 || c[0].Weight != -1 {
		t.Fatalf("NegativeCycle() = %v, want the single self-loop", c)
	}
}

// TestNilAndZeroValue pins the tolerant semantics: a nil or zero-value
// query object answers every question as "no paths".
func TestNilAndZeroValue(t *testing.T) {
	var nilFW *FloydWarshall
	zero := &FloydWarshall{}
	for name, fw := range map[string]*FloydWarshall{"nil": nilFW, "zero": zero} {
		if got := fw.Dist(0, 1); !math.IsInf(got, 1) {
			t.Errorf("%s: Dist = %v, want +Inf", name, got)
		}
		if fw.HasPath(0, 1) {
			t.Errorf("%s: HasPath = true, want false", name)
		}
		if p := fw.Path(0, 1); p != nil {
			t.Errorf("%s: Path = %v, want nil", name, p)
		}
		if fw.HasNegativeCycle() {
			t.Errorf("%s: HasNegativeCycle = true, want false", name)
		}
		if c := fw.NegativeCycle(); c != nil {
			t.Errorf("%s: NegativeCycle = %v, want nil", name, c)
		}
		fw.Lock()   // no-op, must not panic
		fw.Unlock() // no-op, must not panic
	}

	// Out-of-range vertices on a real query object read as no-path.
	g := dijkstra.NewEdgeWeightedDigraph(2)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	fw := NewFloydWarshall(g)
	for _, pair := range [][2]int{{-1, 0}, {0, -1}, {2, 0}, {0, 2}, {99, 99}} {
		if got := fw.Dist(pair[0], pair[1]); !math.IsInf(got, 1) {
			t.Errorf("Dist(%d, %d) = %v, want +Inf (out of range)", pair[0], pair[1], got)
		}
		if fw.HasPath(pair[0], pair[1]) {
			t.Errorf("HasPath(%d, %d) = true, want false (out of range)", pair[0], pair[1])
		}
		if p := fw.Path(pair[0], pair[1]); p != nil {
			t.Errorf("Path(%d, %d) = %v, want nil (out of range)", pair[0], pair[1], p)
		}
	}
}

// TestPanics pins the constructor's panic contract.
func TestPanics(t *testing.T) {
	expectPanic(t, "nil graph", "floyd_warshall: NewFloydWarshall called on a nil or empty graph", func() {
		NewFloydWarshall(nil)
	})
	expectPanic(t, "zero-value graph", "nil or empty graph", func() {
		NewFloydWarshall(&dijkstra.EdgeWeightedDigraph{})
	})
	for name, w := range map[string]float64{
		"NaN":  math.NaN(),
		"+Inf": math.Inf(1),
		"-Inf": math.Inf(-1),
	} {
		g := dijkstra.NewEdgeWeightedDigraph(2)
		g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: w})
		expectPanic(t, name+" weight", "non-finite weight", func() {
			NewFloydWarshall(g)
		})
	}
}

// TestSnapshotImmutability verifies that the query object is a snapshot:
// edges added after construction do not appear in the answers.
func TestSnapshotImmutability(t *testing.T) {
	g := dijkstra.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 1})
	fw := NewFloydWarshall(g)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 0.5}) // a shortcut, added late
	if got, want := fw.Dist(0, 2), 2.0; got != want {
		t.Fatalf("Dist(0, 2) = %v, want %v (late AddEdge must not be reflected)", got, want)
	}
}
