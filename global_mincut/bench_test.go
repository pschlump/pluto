package global_mincut

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// benchGraph builds the shared benchmark input: a sparse random graph
// with integer weights.
func benchGraph(n, m int) *dijkstra.EdgeWeightedGraph {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	g := dijkstra.NewEdgeWeightedGraph(n)
	for i := 0; i < m; i++ {
		g.AddEdge(dijkstra.Edge{V: rng.Intn(n), W: rng.Intn(n), Weight: float64(1 + rng.Intn(10))})
	}
	return g
}

// The backlog question — Stoer–Wagner vs n−1 max-flows — settled on the
// M4 Max: at n=60/m=180 Stoer–Wagner builds in ~300µs vs the max-flow
// route's ~1.7ms (5.7×); at n=150/m=450, ~4.2ms vs ~12.8ms (3×) — and
// the max-flow side's cost is dominated by constructing n−1 FlowNetworks
// (40 MB / 372k allocs at n=150, vs one V×V matrix).  Stoer–Wagner also
// needs no source pair.  The max-flow side lives in thorough_test.go's
// maxflowMinCutBench.
func BenchmarkGlobalMincutStoerWagner60(b *testing.B) {
	g := benchGraph(60, 180)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGlobalMincut(g)
	}
}

func BenchmarkGlobalMincutMaxflow60(b *testing.B) {
	g := benchGraph(60, 180)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if c := maxflowMinCutBench(g); c < 0 {
			b.Fatal("unexpected negative cut")
		}
	}
}

// The matrix form's larger sizes: cubic in n, independent of density
// after the O(V²) build.
func BenchmarkGlobalMincutStoerWagner150(b *testing.B) {
	g := benchGraph(150, 450)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGlobalMincut(g)
	}
}

func BenchmarkGlobalMincutStoerWagner400(b *testing.B) {
	g := benchGraph(400, 1200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGlobalMincut(g)
	}
}

func BenchmarkGlobalMincutMaxflow150(b *testing.B) {
	g := benchGraph(150, 450)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if c := maxflowMinCutBench(g); c < 0 {
			b.Fatal("unexpected negative cut")
		}
	}
}
