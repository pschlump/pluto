package mst

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// -------------------------------------------------------------------------------------------------------
// Independent reference: brute-force Prim over plain slices
// -------------------------------------------------------------------------------------------------------

// refEdge is a plain-slice edge for the reference implementation, which
// must not import other pluto packages (the graph, the union-find and
// the priority queues are all under test here).
type refEdge struct {
	v, w int
	wt   float64
}

// referenceMSTWeight computes the total weight of the minimum spanning
// forest by a naive O(V·E) Prim: repeatedly scan ALL edges for the
// lightest one leaving the tree.  Slow, obviously correct, and fully
// independent of the priority-queue and union-find machinery under
// test.  All weights in these tests are small integers stored in
// float64, so every sum is exact and totals compare with ==.
func referenceMSTWeight(n int, edges []refEdge) float64 {
	inTree := make([]bool, n)
	total := 0.0
	for s := 0; s < n; s++ { // restart per component: spanning FOREST
		if inTree[s] {
			continue
		}
		inTree[s] = true
		for {
			best := -1 // the non-tree endpoint of the lightest crossing edge, -1 if none
			bestW := 0.0
			for _, e := range edges {
				if e.v == e.w {
					continue // a self-loop never joins a tree
				}
				var o int
				switch {
				case inTree[e.v] && !inTree[e.w]:
					o = e.w
				case inTree[e.w] && !inTree[e.v]:
					o = e.v
				default:
					continue
				}
				if best == -1 || e.wt < bestW {
					best, bestW = o, e.wt
				}
			}
			if best == -1 {
				break
			}
			inTree[best] = true
			total += bestW
		}
	}
	return total
}

// -------------------------------------------------------------------------------------------------------
// Randomized verification against the reference (fixed seed)
// -------------------------------------------------------------------------------------------------------

// benchmarkGraph builds a random connected-ish graph for benchmarks.
func benchmarkGraph(n, m int, seed int64) *dijkstra.EdgeWeightedGraph {
	rng := rand.New(rand.NewSource(seed))
	g := dijkstra.NewEdgeWeightedGraph(n)
	for v := 0; v+1 < n; v++ { // a spanning path so the graph is connected
		g.AddEdge(dijkstra.Edge{V: v, W: v + 1, Weight: float64(rng.Intn(100) + 1)})
	}
	for i := 0; i < m; i++ {
		g.AddEdge(dijkstra.Edge{V: rng.Intn(n), W: rng.Intn(n), Weight: float64(rng.Intn(100) + 1)})
	}
	return g
}

// TestMSTRandomizedAgreement runs all three algorithms on many small
// random graphs — parallel edges, self-loops and negative weights
// included — and cross-checks: (a) all three weights against each
// other, (b) every weight against the independent brute-force
// reference, and (c) the forest property of every edge set.  Weights
// are integers in float64, so all comparisons are exact.
func TestMSTRandomizedAgreement(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	for trial := range 300 {
		n := 1 + rng.Intn(12)
		m := rng.Intn(4 * n)
		g := dijkstra.NewEdgeWeightedGraph(n)
		var edges []refEdge
		for range m {
			e := refEdge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(41) - 20)} // -20..20, negatives legal
			g.AddEdge(dijkstra.Edge{V: e.v, W: e.w, Weight: e.wt})
			edges = append(edges, e)
		}

		want := referenceMSTWeight(n, edges)

		lp := NewLazyPrimMST(g)
		pr := NewPrimMST(g)
		kr := NewKruskalMST(g)

		if lp.Weight() != want {
			t.Fatalf("trial %d: LazyPrimMST weight %v, reference says %v", trial, lp.Weight(), want)
		}
		if pr.Weight() != want {
			t.Fatalf("trial %d: PrimMST weight %v, reference says %v", trial, pr.Weight(), want)
		}
		if kr.Weight() != want {
			t.Fatalf("trial %d: KruskalMST weight %v, reference says %v", trial, kr.Weight(), want)
		}

		checkForest(t, "LazyPrimMST", n, g, lp.Edges())
		checkForest(t, "PrimMST", n, g, pr.Edges())
		checkForest(t, "KruskalMST", n, g, kr.Edges())
		if lp.Len() != len(lp.Edges()) || pr.Len() != len(pr.Edges()) || kr.Len() != len(kr.Edges()) {
			t.Fatalf("trial %d: Len disagrees with len(Edges)", trial)
		}
	}
}

// TestMSTEdgesWeightConsistency checks on random graphs that Weight()
// always equals the sum of the returned edge weights, and that Edges()
// returns a fresh slice (mutating it must not corrupt the result).
func TestMSTEdgesWeightConsistency(t *testing.T) {
	rng := rand.New(rand.NewSource(43)) // fixed seed: deterministic run

	for trial := range 100 {
		n := 1 + rng.Intn(10)
		m := rng.Intn(4 * n)
		g := dijkstra.NewEdgeWeightedGraph(n)
		for range m {
			g.AddEdge(dijkstra.Edge{V: rng.Intn(n), W: rng.Intn(n), Weight: float64(rng.Intn(20) + 1)})
		}

		kr := NewKruskalMST(g)
		sum := 0.0
		for _, e := range kr.Edges() {
			sum += e.Weight
		}
		if sum != kr.Weight() {
			t.Fatalf("trial %d: Edges sum to %v, Weight reports %v", trial, sum, kr.Weight())
		}

		// Mutating the returned slice must not change the result object.
		edges := kr.Edges()
		if len(edges) > 0 {
			edges[0].Weight = -1000
		}
		if kr.Weight() != sum {
			t.Fatalf("trial %d: mutating the Edges slice changed Weight to %v (was %v)", trial, kr.Weight(), sum)
		}
	}
}
