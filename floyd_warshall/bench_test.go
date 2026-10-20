/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package floyd_warshall

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// denseDigraph builds a dense random digraph (each ordered pair present
// with probability 0.8) with non-negative integer weights, so that
// dijkstra's all-pairs search — the sparse-graph alternative — is also
// a legal comparison.
func denseDigraph(n int, seed1, seed2 uint64) *dijkstra.EdgeWeightedDigraph {
	rng := rand.New(rand.NewPCG(seed1, seed2))
	g := dijkstra.NewEdgeWeightedDigraph(n)
	for v := range n {
		for w := range n {
			if v != w && rng.Float64() < 0.8 {
				g.AddEdge(dijkstra.DirectedEdge{From: v, To: w, Weight: float64(1 + rng.IntN(1000))})
			}
		}
	}
	return g
}

// The density trade-off of the two all-pairs algorithms: Floyd-Warshall
// is O(V³) regardless of E; V Dijkstras are O(V·E log V).  At E ≈ V²
// the Dijkstra work is O(V³ log V) — Floyd-Warshall's home turf.
func BenchmarkFloydWarshall(b *testing.B) {
	for _, n := range []int{100, 250, 500} {
		g := denseDigraph(n, 42, 7)
		b.Run("V="+strconv.Itoa(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				fw := NewFloydWarshall(g)
				if fw.HasNegativeCycle() {
					b.Fatal("unexpected negative cycle in a non-negative benchmark graph")
				}
			}
		})
	}
}

func BenchmarkDijkstraAllPairs(b *testing.B) {
	for _, n := range []int{100, 250, 500} {
		g := denseDigraph(n, 42, 7)
		b.Run("V="+strconv.Itoa(n), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ap := dijkstra.NewDijkstraAllPairsSP(g)
				if !ap.HasPath(0, n-1) {
					b.Fatal("unexpected unreachable pair in a dense benchmark graph")
				}
			}
		})
	}
}

// BenchmarkQuery measures the O(1) matrix lookups after construction.
func BenchmarkQuery(b *testing.B) {
	g := denseDigraph(250, 42, 7)
	fw := NewFloydWarshall(g)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for v := range 250 {
			_ = fw.Dist(v, (v*7)%250)
			_ = fw.HasPath(v, (v*13)%250)
		}
	}
}
