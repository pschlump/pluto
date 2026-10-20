package floyd_warshall_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/pschlump/pluto/dijkstra_ts"
)

// -------------------------------------------------------------------------------------------------------
// Independent reference: repeated relaxation over plain int slices
// (NO pluto imports in the reference — the package under test is
// checked against a model that shares no code with it)
// -------------------------------------------------------------------------------------------------------

// refEdge is the reference model's edge type: integer weights, stored
// and summed exactly, so distances compare with ==.
type refEdge struct {
	from, to, wt int
}

// referenceHasNegativeCycle decides whether the graph has a negative
// cycle ANYWHERE by running Bellman-Ford from a virtual super source
// with a zero-weight edge to every vertex: if the n-th round of
// relaxing every edge still improves anything, a negative cycle exists
// somewhere.
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

// TestFloydWarshallRandomized mirrors the plain package's randomized
// cross-check (fixed seed 42/7): random small digraphs with integer
// weights including negatives; negative-cycle detection must agree with
// the super-source reference, and on clean instances every distance is
// compared exactly, with paths checked for connectivity and exact
// weight sums.
func TestFloydWarshallRandomized(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	ranges := [][2]int{{0, 10}, {-5, 5}, {-10, 10}, {1, 4}, {-3, 12}}
	cycles, noCycles := 0, 0
	for iter := range 300 {
		r := ranges[iter%len(ranges)]
		n := 1 + rng.IntN(12)
		g := dijkstra_ts.NewEdgeWeightedDigraph(n)
		var edges []refEdge
		for v := range n {
			for w := range n {
				if v != w && rng.Float64() < 0.1+rng.Float64()*0.8 {
					wt := r[0] + rng.IntN(r[1]-r[0]+1)
					g.AddEdge(dijkstra_ts.DirectedEdge{From: v, To: w, Weight: float64(wt)})
					edges = append(edges, refEdge{v, w, wt})
				}
			}
		}

		fw := NewFloydWarshall(g)
		if ref := referenceHasNegativeCycle(n, edges); fw.HasNegativeCycle() != ref {
			t.Fatalf("trial %d: HasNegativeCycle() = %v, reference says %v", iter, fw.HasNegativeCycle(), ref)
		}
		if fw.HasNegativeCycle() {
			cycles++
			cycle := fw.NegativeCycle()
			if len(cycle) == 0 {
				t.Fatalf("trial %d: NegativeCycle() returned no edges", iter)
			}
			sum := 0.0
			for i, e := range cycle {
				if next := cycle[(i+1)%len(cycle)]; e.To != next.From {
					t.Fatalf("trial %d: NegativeCycle() is not a cycle at edge %d", iter, i)
				}
				sum += e.Weight
			}
			if sum >= 0 {
				t.Fatalf("trial %d: NegativeCycle() weights sum to %v, expected negative", iter, sum)
			}
			continue
		}
		noCycles++
		want := referenceAllPairs(n, edges)
		for v := range n {
			for w := range n {
				if got := fw.Dist(v, w); got != want[v][w] {
					t.Fatalf("trial %d: Dist(%d, %d) = %v, reference says %v", iter, v, w, got, want[v][w])
				}
				if fw.HasPath(v, w) != !math.IsInf(want[v][w], 1) {
					t.Fatalf("trial %d: HasPath(%d, %d) disagrees with the reference distance %v",
						iter, v, w, want[v][w])
				}
				if !fw.HasPath(v, w) || v == w {
					continue
				}
				path := fw.Path(v, w)
				if len(path) == 0 || path[0].From != v || path[len(path)-1].To != w {
					t.Fatalf("trial %d: Path(%d, %d) = %v — wrong endpoints", iter, v, w, path)
				}
				sum := 0.0
				for i, e := range path {
					if i > 0 && path[i-1].To != e.From {
						t.Fatalf("trial %d: Path(%d, %d) is not connected at edge %d", iter, v, w, i)
					}
					sum += e.Weight
				}
				if sum != fw.Dist(v, w) {
					t.Fatalf("trial %d: Path(%d, %d) sums to %v, Dist = %v", iter, v, w, sum, fw.Dist(v, w))
				}
			}
		}
	}
	if cycles == 0 || noCycles == 0 {
		t.Fatalf("oracle coverage: expected both branches, got %d negative-cycle and %d clean instances", cycles, noCycles)
	}
}

// -------------------------------------------------------------------------------------------------------
// Concurrency smoke test (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestFloydWarshallConcurrent hammers a digraph with concurrent AddEdge
// writers, V/E/Adj readers, FloydWarshall constructors (which snapshot
// under the read lock), and readers of the immutable query object — all
// at once.  Correctness is verified single-threaded afterwards; under
// -race this fails on any data race.
func TestFloydWarshallConcurrent(t *testing.T) {
	const n = 16

	// A weight-1 chain 0->1->...->15, so the initial query object has a
	// known distance to check afterwards.
	g := dijkstra_ts.NewEdgeWeightedDigraph(n)
	for v := range n - 1 {
		g.AddEdge(dijkstra_ts.DirectedEdge{From: v, To: v + 1, Weight: 1})
	}
	fw := NewFloydWarshall(g)

	var wg sync.WaitGroup

	// Readers: the immutable query object plus the graph read operations.
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				for v := range n {
					for w := range n {
						fw.Dist(v, w)
						fw.HasPath(v, w)
						fw.Path(v, w)
					}
					fw.HasNegativeCycle()
					fw.NegativeCycle()
					g.V()
					g.E()
					g.Adj(v)
				}
			}
		}()
	}

	// Writers: non-negative weights only, so concurrent constructors
	// never find a negative cycle (which would panic the readers'
	// Dist).
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 500 {
				g.AddEdge(dijkstra_ts.DirectedEdge{From: i % n, To: (i + w + 1) % n, Weight: 1})
			}
		}(w)
	}

	// Constructors running concurrently with the writers — each
	// snapshots the adjacency under the read lock, then computes
	// lock-free.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			NewFloydWarshall(g)
		}
	}()

	wg.Wait()

	// The pre-concurrency query object is an immutable snapshot: the
	// chain distance must be exactly n-1 regardless of the writes.
	if fw.Dist(0, n-1) != float64(n-1) {
		t.Errorf("Dist(0, %d) = %v, expected %d (immutable snapshot)", n-1, fw.Dist(0, n-1), n-1)
	}
	if fw.HasNegativeCycle() {
		t.Errorf("HasNegativeCycle() = true on a non-negative-weight digraph.")
	}
}
