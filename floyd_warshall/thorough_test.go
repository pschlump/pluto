package floyd_warshall

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// -------------------------------------------------------------------------------------------------------
// Independent reference: per-source repeated relaxation over plain int
// slices (NO pluto imports in the reference — the package under test is
// checked against a model that shares no code with it)
// -------------------------------------------------------------------------------------------------------

// refEdge is the reference model's edge type: integer weights, stored
// and summed exactly, so distances compare with ==.
type refEdge struct {
	from, to, wt int
}

// referenceHasNegativeCycle decides whether the graph has a negative
// cycle ANYWHERE by running Bellman-Ford from a virtual super source
// with a zero-weight edge to every vertex (all distances start at 0):
// if the n-th round of relaxing every edge still improves anything, a
// negative cycle exists somewhere.
func referenceHasNegativeCycle(n int, edges []refEdge) bool {
	dist := make([]float64, n)
	for round := range n {
		changed := false
		for _, e := range edges {
			if nd := dist[e.from] + float64(e.wt); nd < dist[e.to] {
				dist[e.to] = nd
				changed = true
			}
		}
		if !changed {
			return false
		}
		if round == n-1 {
			return true // the n-th round still improved
		}
	}
	return true
}

// referenceAllPairs computes the all-pairs distance matrix by n-1 rounds
// of relaxing every edge per source — O(V²·E), slow and sure.  Only
// called on graphs with no negative cycle.
func referenceAllPairs(n int, edges []refEdge) [][]float64 {
	dist := make([][]float64, n)
	for s := range n {
		dist[s] = make([]float64, n)
		for v := range n {
			dist[s][v] = math.Inf(1)
		}
		dist[s][s] = 0
		for range n - 1 {
			changed := false
			for _, e := range edges {
				if nd := dist[s][e.from] + float64(e.wt); nd < dist[s][e.to] {
					dist[s][e.to] = nd
					changed = true
				}
			}
			if !changed {
				break
			}
		}
	}
	return dist
}

// checkMatrixCertificate verifies the all-pairs optimality certificate
// on exact integer instances: the diagonal is 0, no distance can be
// improved by routing through ANY intermediate vertex (the triangle
// inequality over all n³ triples — this alone proves the matrix optimal,
// independent of the reference), and no direct edge beats it.
func checkMatrixCertificate(t *testing.T, name string, fw *FloydWarshall, g *dijkstra.EdgeWeightedDigraph) {
	t.Helper()
	n := g.V()
	for v := range n {
		if fw.Dist(v, v) != 0 {
			t.Fatalf("%s: Dist(%d, %d) = %v, want 0", name, v, v, fw.Dist(v, v))
		}
		for w := range n {
			if !fw.HasPath(v, w) {
				continue
			}
			for u := range n {
				if !fw.HasPath(v, u) || !fw.HasPath(u, w) {
					continue
				}
				if via := fw.Dist(v, u) + fw.Dist(u, w); via < fw.Dist(v, w) {
					t.Fatalf("%s: Dist(%d, %d) = %v but via %d it is %v (not optimal)",
						name, v, w, fw.Dist(v, w), u, via)
				}
			}
		}
	}
	for v := range n {
		for _, e := range g.Adj(v) {
			if e.Weight < fw.Dist(v, e.To) {
				t.Fatalf("%s: direct edge %v beats Dist(%d, %d) = %v", name, e, v, e.To, fw.Dist(v, e.To))
			}
		}
	}
}

// crossCheckModel drives one random instance through both the package
// and the reference: negative-cycle agreement (with the cycle
// certificate) or the full n² distance matrix compared exactly (integer
// weights keep every sum exact in float64), plus the optimality
// certificate and path checks on reachable pairs.  It reports whether a
// negative cycle was found.
func crossCheckModel(t *testing.T, name string, n int, edges []refEdge, g *dijkstra.EdgeWeightedDigraph) (hasNegCycle bool) {
	t.Helper()
	fw := NewFloydWarshall(g)
	if ref := referenceHasNegativeCycle(n, edges); fw.HasNegativeCycle() != ref {
		t.Fatalf("%s: HasNegativeCycle() = %v, reference says %v", name, fw.HasNegativeCycle(), ref)
	}
	if fw.HasNegativeCycle() {
		checkCycle(t, name, g, fw.NegativeCycle())
		return true
	}
	want := referenceAllPairs(n, edges)
	for v := range n {
		for w := range n {
			if got := fw.Dist(v, w); got != want[v][w] {
				t.Fatalf("%s: Dist(%d, %d) = %v, reference says %v", name, v, w, got, want[v][w])
			}
			if fw.HasPath(v, w) != !math.IsInf(want[v][w], 1) {
				t.Fatalf("%s: HasPath(%d, %d) disagrees with the reference distance %v", name, v, w, want[v][w])
			}
			if fw.HasPath(v, w) {
				checkPath(t, name, fw, g, v, w)
			} else if p := fw.Path(v, w); p != nil {
				t.Fatalf("%s: Path(%d, %d) = %v for an unreachable pair", name, v, w, p)
			}
		}
	}
	checkMatrixCertificate(t, name, fw, g)
	return false
}

// randomGraph builds a random digraph with integer weights in [lo, hi]
// and edge probability p, recording the edges in the reference's form.
func randomGraph(rng *rand.Rand, n int, p float64, lo, hi int) (*dijkstra.EdgeWeightedDigraph, []refEdge) {
	g := dijkstra.NewEdgeWeightedDigraph(n)
	var edges []refEdge
	for v := range n {
		for w := range n {
			if v == w || rng.Float64() >= p {
				continue
			}
			wt := lo + rng.IntN(hi-lo+1)
			g.AddEdge(dijkstra.DirectedEdge{From: v, To: w, Weight: float64(wt)})
			edges = append(edges, refEdge{v, w, wt})
		}
	}
	return g, edges
}

// TestRandomizedModel: random digraphs across density and weight ranges
// (negatives included) at fixed seed 42/7 — every instance exercises
// either the negative-cycle branch or the full-matrix branch, both of
// which must occur.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	ranges := [][2]int{{0, 10}, {-5, 5}, {-10, 10}, {1, 4}, {-3, 12}}
	cycles, noCycles := 0, 0
	for iter := range 300 {
		r := ranges[iter%len(ranges)]
		n := 1 + rng.IntN(13)
		p := 0.1 + rng.Float64()*0.8
		g, edges := randomGraph(rng, n, p, r[0], r[1])
		if crossCheckModel(t, "random", n, edges, g) {
			cycles++
		} else {
			noCycles++
		}
	}
	if cycles == 0 || noCycles == 0 {
		t.Fatalf("oracle coverage: expected both branches, got %d negative-cycle and %d clean instances", cycles, noCycles)
	}
}

// TestDAGNegativeWeights: random DAGs (edges only from lower to higher
// vertices) with negative weights — acyclicity guarantees no negative
// cycle, so every instance takes the full-matrix branch with plenty of
// negative distances in play.
func TestDAGNegativeWeights(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 42))
	for range 150 {
		n := 1 + rng.IntN(13)
		g := dijkstra.NewEdgeWeightedDigraph(n)
		var edges []refEdge
		for v := range n {
			for w := v + 1; w < n; w++ {
				if rng.Float64() < 0.4 {
					wt := -10 + rng.IntN(21)
					g.AddEdge(dijkstra.DirectedEdge{From: v, To: w, Weight: float64(wt)})
					edges = append(edges, refEdge{v, w, wt})
				}
			}
		}
		crossCheckModel(t, "dag", n, edges, g)
	}
}

// TestFloatWeights: non-integral weights (negatives included) compared
// against the same reference with a small tolerance — every weight is an
// exact multiple of 1/8, so all sums stay exact in binary floating
// point and agreement is tight.  The package does no rounding.
func TestFloatWeights(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	for iter := range 150 {
		n := 1 + rng.IntN(9)
		g := dijkstra.NewEdgeWeightedDigraph(n)
		var edges []refEdge
		for v := range n {
			for w := range n {
				if v != w && rng.Float64() < 0.5 {
					wt := rng.IntN(81) - 40 // weight wt/8 in [-5, 5]
					g.AddEdge(dijkstra.DirectedEdge{From: v, To: w, Weight: float64(wt) / 8})
					edges = append(edges, refEdge{v, w, wt})
				}
			}
		}
		fw := NewFloydWarshall(g)
		if ref := referenceHasNegativeCycle(n, edges); fw.HasNegativeCycle() != ref {
			t.Fatalf("iter %d: HasNegativeCycle() = %v, reference says %v", iter, fw.HasNegativeCycle(), ref)
		}
		if fw.HasNegativeCycle() {
			checkCycle(t, "float", g, fw.NegativeCycle())
			continue
		}
		want := referenceAllPairs(n, edges)
		for v := range n {
			for w := range n {
				got := fw.Dist(v, w)
				ref := want[v][w] / 8
				if math.Abs(got-ref) > 1e-9 {
					t.Fatalf("iter %d: Dist(%d, %d) = %v, reference says %v", iter, v, w, got, ref)
				}
			}
		}
	}
}

// TestLargerDenseInstance: past any teaching example — a dense graph
// where Floyd-Warshall is the algorithm of choice, integer weights so
// the matrix comparison stays exact.
func TestLargerDenseInstance(t *testing.T) {
	rng := rand.New(rand.NewPCG(2026, 10))
	n := 150
	g, edges := randomGraph(rng, n, 0.8, -50, 50)
	fw := NewFloydWarshall(g)
	if ref := referenceHasNegativeCycle(n, edges); fw.HasNegativeCycle() != ref {
		t.Fatalf("HasNegativeCycle() = %v, reference says %v", fw.HasNegativeCycle(), ref)
	}
	if fw.HasNegativeCycle() {
		checkCycle(t, "dense", g, fw.NegativeCycle())
		return
	}
	want := referenceAllPairs(n, edges)
	for v := range n {
		for w := range n {
			if got := fw.Dist(v, w); got != want[v][w] {
				t.Fatalf("Dist(%d, %d) = %v, reference says %v", v, w, got, want[v][w])
			}
		}
	}
	checkMatrixCertificate(t, "dense", fw, g)
}

// TestDisconnectedAndSparse: graphs with isolated vertices and very few
// edges — unreachable pairs must read +Inf / false / nil on both sides.
func TestDisconnectedAndSparse(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 100 {
		n := 2 + rng.IntN(10)
		g := dijkstra.NewEdgeWeightedDigraph(n)
		var edges []refEdge
		for range rng.IntN(3) {
			v, w := rng.IntN(n), rng.IntN(n)
			wt := rng.IntN(11)
			g.AddEdge(dijkstra.DirectedEdge{From: v, To: w, Weight: float64(wt)})
			edges = append(edges, refEdge{v, w, wt})
		}
		crossCheckModel(t, "sparse", n, edges, g)
	}
}
