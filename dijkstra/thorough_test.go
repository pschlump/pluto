package dijkstra

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
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
