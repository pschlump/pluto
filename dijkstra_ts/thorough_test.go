package dijkstra_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"sync"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Independent reference: repeated-relaxation (Bellman-Ford style)
// -------------------------------------------------------------------------------------------------------

// referenceDigraphDist computes shortest distances from s by n-1 rounds
// of relaxing every edge — the slow-and-sure Bellman-Ford reference,
// independent of the priority-queue machinery under test.  All weights
// in these tests are small integers stored in float64, so every sum is
// exact and distances compare with ==.
func referenceDigraphDist(n int, edges []DirectedEdge, s int) []float64 {
	dist := make([]float64, n)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[s] = 0
	for range n - 1 {
		for _, e := range edges {
			if nd := dist[e.From] + e.Weight; nd < dist[e.To] {
				dist[e.To] = nd
			}
		}
	}
	return dist
}

// referenceGraphDist is the undirected twin: every edge relaxes in both
// directions.
func referenceGraphDist(n int, edges []Edge, s int) []float64 {
	dist := make([]float64, n)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[s] = 0
	for range n - 1 {
		for _, e := range edges {
			if nd := dist[e.V] + e.Weight; nd < dist[e.W] {
				dist[e.W] = nd
			}
			if nd := dist[e.W] + e.Weight; nd < dist[e.V] {
				dist[e.V] = nd
			}
		}
	}
	return dist
}

// -------------------------------------------------------------------------------------------------------
// Randomized verification against the reference (fixed seed)
// -------------------------------------------------------------------------------------------------------

// TestDijkstraSPRandomized runs Dijkstra on many random small digraphs
// (non-negative integer weights, self-loops and parallel edges included)
// and compares every distance — reachable and +Inf alike — against the
// Bellman-Ford reference, exactly.
func TestDijkstraSPRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	for trial := range 200 {
		n := 1 + rng.Intn(12)
		m := rng.Intn(4 * n)
		g := NewEdgeWeightedDigraph(n)
		var edges []DirectedEdge
		for range m {
			e := DirectedEdge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(20) + 1)}
			g.AddEdge(e)
			edges = append(edges, e)
		}

		s := rng.Intn(n)
		want := referenceDigraphDist(n, edges, s)
		sp := NewDijkstraSP(g, s)

		for v := range n {
			if sp.DistTo(v) != want[v] {
				t.Fatalf("trial %d: DistTo(%d) = %v, reference says %v", trial, v, sp.DistTo(v), want[v])
			}
			if sp.HasPathTo(v) != !math.IsInf(want[v], 1) {
				t.Fatalf("trial %d: HasPathTo(%d) = %v, reference distance is %v",
					trial, v, sp.HasPathTo(v), want[v])
			}
			// The reconstructed path must be connected, start at s, end
			// at v, and sum to DistTo(v) exactly.
			if math.IsInf(want[v], 1) || v == s {
				continue
			}
			path := sp.PathTo(v)
			if len(path) == 0 || path[0].From != s || path[len(path)-1].To != v {
				t.Fatalf("trial %d: PathTo(%d) = %v — wrong endpoints", trial, v, path)
			}
			sum := 0.0
			for i, e := range path {
				if i > 0 && path[i-1].To != e.From {
					t.Fatalf("trial %d: PathTo(%d) is not connected at edge %d", trial, v, i)
				}
				sum += e.Weight
			}
			if sum != sp.DistTo(v) {
				t.Fatalf("trial %d: PathTo(%d) sums to %v, DistTo = %v", trial, v, sum, sp.DistTo(v))
			}
		}
	}
}

// TestDijkstraUndirectedSPRandomized is the undirected twin of the
// randomized cross-check.
func TestDijkstraUndirectedSPRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(43)) // fixed seed: deterministic run

	for trial := range 200 {
		n := 1 + rng.Intn(12)
		m := rng.Intn(4 * n)
		g := NewEdgeWeightedGraph(n)
		var edges []Edge
		for range m {
			e := Edge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(20) + 1)}
			g.AddEdge(e)
			edges = append(edges, e)
		}

		s := rng.Intn(n)
		want := referenceGraphDist(n, edges, s)
		sp := NewDijkstraUndirectedSP(g, s)

		for v := range n {
			if sp.DistTo(v) != want[v] {
				t.Fatalf("trial %d: DistTo(%d) = %v, reference says %v", trial, v, sp.DistTo(v), want[v])
			}
			if sp.HasPathTo(v) != !math.IsInf(want[v], 1) {
				t.Fatalf("trial %d: HasPathTo(%d) = %v, reference distance is %v",
					trial, v, sp.HasPathTo(v), want[v])
			}
			if math.IsInf(want[v], 1) || v == s {
				continue
			}
			path := sp.PathTo(v)
			if len(path) == 0 {
				t.Fatalf("trial %d: PathTo(%d) returned no edges", trial, v)
			}
			sum := 0.0
			at := s
			for _, e := range path {
				next := e.Other(at)
				if next == at {
					t.Fatalf("trial %d: PathTo(%d) edge %v does not advance from %d", trial, v, e, at)
				}
				at = next
				sum += e.Weight
			}
			if at != v {
				t.Fatalf("trial %d: PathTo(%d) ends at %d", trial, v, at)
			}
			if sum != sp.DistTo(v) {
				t.Fatalf("trial %d: PathTo(%d) sums to %v, DistTo = %v", trial, v, sum, sp.DistTo(v))
			}
		}
	}
}

// TestDijkstraAllPairsSPRandomized checks the all-pairs query object
// against a fresh single-source computation for every (s, t) pair.
func TestDijkstraAllPairsSPRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(44)) // fixed seed: deterministic run

	for trial := range 20 {
		n := 1 + rng.Intn(10)
		m := rng.Intn(4 * n)
		g := NewEdgeWeightedDigraph(n)
		for range m {
			g.AddEdge(DirectedEdge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(20) + 1)})
		}

		ap := NewDijkstraAllPairsSP(g)
		for s := range n {
			sp := NewDijkstraSP(g, s)
			for v := range n {
				if ap.Dist(s, v) != sp.DistTo(v) {
					t.Fatalf("trial %d: Dist(%d,%d) = %v, single-source says %v",
						trial, s, v, ap.Dist(s, v), sp.DistTo(v))
				}
				if ap.HasPath(s, v) != sp.HasPathTo(v) {
					t.Fatalf("trial %d: HasPath(%d,%d) disagrees with HasPathTo", trial, s, v)
				}
			}
		}
	}
}

// TestDijkstraSPTriangleInequality checks the optimality condition on a
// larger random digraph: for every source s and every edge u -> t,
// dist(s, t) <= dist(s, u) + w(u, t).
func TestDijkstraSPTriangleInequality(t *testing.T) {
	rng := rand.New(rand.NewSource(45)) // fixed seed: deterministic run

	const n = 60
	const m = 300
	g := NewEdgeWeightedDigraph(n)
	var edges []DirectedEdge
	for range m {
		e := DirectedEdge{rng.Intn(n), rng.Intn(n), float64(rng.Intn(20) + 1)}
		g.AddEdge(e)
		edges = append(edges, e)
	}

	for s := 0; s < n; s += 7 { // a sample of sources is enough
		sp := NewDijkstraSP(g, s)
		for _, e := range edges {
			du := sp.DistTo(e.From)
			if math.IsInf(du, 1) {
				continue // +Inf on the right side: the inequality holds trivially
			}
			if sp.DistTo(e.To) > du+e.Weight {
				t.Fatalf("triangle inequality violated: dist(%d,%d)=%v > dist(%d,%d)+%v=%v",
					s, e.To, sp.DistTo(e.To), s, e.From, e.Weight, du+e.Weight)
			}
		}
	}
}

// -------------------------------------------------------------------------------------------------------
// Concurrency smoke test (run with -race)
// -------------------------------------------------------------------------------------------------------

// TestDijkstraConcurrent hammers the graphs with concurrent AddEdge
// writers, V/E/Adj readers, Dijkstra constructors (which snapshot under
// the read lock), and readers of the immutable query objects — all at
// once.  Correctness is verified single-threaded afterwards; under
// -race this fails on any data race.
func TestDijkstraConcurrent(t *testing.T) {
	const n = 16

	// A weight-1 chain 0->1->...->15, so the initial query object has a
	// known distance to check afterwards.
	g := NewEdgeWeightedDigraph(n)
	ug := NewEdgeWeightedGraph(n)
	for v := range n - 1 {
		g.AddEdge(DirectedEdge{v, v + 1, 1})
		ug.AddEdge(Edge{v, v + 1, 1})
	}
	sp := NewDijkstraSP(g, 0)
	usp := NewDijkstraUndirectedSP(ug, 0)
	ap := NewDijkstraAllPairsSP(g)

	var wg sync.WaitGroup

	// Readers: immutable query objects plus the graph read operations.
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				for v := range n {
					sp.DistTo(v)
					sp.HasPathTo(v)
					sp.PathTo(v)
					usp.DistTo(v)
					usp.PathTo(v)
					ap.Dist(0, v)
					ap.HasPath(v, 0)
					g.V()
					g.E()
					g.Adj(v)
					ug.Adj(v)
				}
			}
		}()
	}

	// Writers: non-negative weights only, so concurrent constructors
	// never hit the negative-weight panic.
	for w := range 2 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 500 {
				g.AddEdge(DirectedEdge{i % n, (i + w + 1) % n, 1})
				ug.AddEdge(Edge{i % n, (i + w + 1) % n, 1})
			}
		}(w)
	}

	// Constructors running concurrently with the writers — each
	// snapshots the adjacency under the read lock, then searches
	// lock-free.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 20 {
			NewDijkstraSP(g, 0)
			NewDijkstraAllPairsSP(g)
			NewDijkstraUndirectedSP(ug, 0)
		}
	}()

	wg.Wait()

	// The pre-concurrency query objects are immutable snapshots: the
	// chain distance must be exactly n-1 regardless of the writes.
	if sp.DistTo(n-1) != float64(n-1) {
		t.Errorf("DistTo(%d) = %v, expected %d (immutable snapshot)", n-1, sp.DistTo(n-1), n-1)
	}
	if usp.DistTo(n-1) != float64(n-1) {
		t.Errorf("undirected DistTo(%d) = %v, expected %d", n-1, usp.DistTo(n-1), n-1)
	}
}

// TestDijkstraLockUnlock exercises the real write lock on the graphs and
// the no-op Lock/Unlock on the query objects.
func TestDijkstraLockUnlock(t *testing.T) {
	g := NewEdgeWeightedDigraph(4)
	g.Lock()
	g.Unlock()
	// After Lock/Unlock the graph is fully usable.
	g.AddEdge(DirectedEdge{0, 1, 1})
	if g.E() != 1 {
		t.Errorf("Expected E=1 after Lock/Unlock/AddEdge, got %d", g.E())
	}

	sp := NewDijkstraSP(g, 0)
	sp.Lock()   // no-op, must not panic
	sp.Unlock() // no-op, must not panic

	var nilG *EdgeWeightedDigraph
	nilG.Lock()   // nil no-op
	nilG.Unlock() // nil no-op
	var nilUG *EdgeWeightedGraph
	nilUG.Lock()
	nilUG.Unlock()
}
