package mst_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/pschlump/pluto/dijkstra_ts"
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
		g := dijkstra_ts.NewEdgeWeightedGraph(n)
		var edges []refEdge
		for range m {
			e := refEdge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(41) - 20)} // -20..20, negatives legal
			g.AddEdge(dijkstra_ts.Edge{V: e.v, W: e.w, Weight: e.wt})
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

// -------------------------------------------------------------------------------------------------------
// Concurrency hammer (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestMSTConcurrent hammers a shared graph with concurrent AddEdge
// writers while other goroutines run the three MST constructors (which
// snapshot the adjacency under the read lock, then compute lock-free)
// and read the immutable result objects.  Correctness is verified
// single-threaded afterwards; under -race this fails on any data race.
func TestMSTConcurrent(t *testing.T) {
	const n = 16

	// A weight-1 spanning chain, so the graph stays connected and the
	// initial query objects have known weights to check afterwards.
	g := dijkstra_ts.NewEdgeWeightedGraph(n)
	for v := 0; v+1 < n; v++ {
		g.AddEdge(dijkstra_ts.Edge{V: v, W: v + 1, Weight: 1})
	}
	lp := NewLazyPrimMST(g)
	pr := NewPrimMST(g)
	kr := NewKruskalMST(g)
	if lp.Weight() != n-1 || pr.Weight() != n-1 || kr.Weight() != n-1 {
		t.Fatalf("Initial MST weights = %v/%v/%v, expected %d", lp.Weight(), pr.Weight(), kr.Weight(), n-1)
	}

	var wg sync.WaitGroup

	// Readers: immutable query objects, the graph read operations, and
	// fresh constructors racing the writers.
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				for v := 0; v < n; v++ {
					g.Adj(v)
				}
				g.V()
				g.E()
				lp.Edges()
				lp.Weight()
				pr.Edges()
				kr.Len()
				NewLazyPrimMST(g).Weight()
				NewPrimMST(g).Len()
				NewKruskalMST(g).Edges()
			}
		}()
	}

	// Writers: any weights are fine — the MST constructors never reject
	// an edge.
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				g.AddEdge(dijkstra_ts.Edge{V: (w + i) % n, W: (w + i + 3) % n, Weight: float64(i%9) - 4})
			}
		}(w)
	}

	wg.Wait()

	// Afterwards, single-threaded: all three algorithms must still
	// agree on the weight of whatever the graph has become.
	w1, w2, w3 := NewLazyPrimMST(g).Weight(), NewPrimMST(g).Weight(), NewKruskalMST(g).Weight()
	if w1 != w2 || w2 != w3 {
		t.Errorf("Post-hammer weights disagree: lazy %v, eager %v, kruskal %v", w1, w2, w3)
	}
	if NewKruskalMST(g).Len() != n-1 { // the writers never disconnect the chain
		t.Errorf("Post-hammer Len = %d, expected %d (connected graph)", NewKruskalMST(g).Len(), n-1)
	}
}
