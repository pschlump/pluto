/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bipartite

import (
	"math/rand/v2"
	"testing"

	"github.com/pschlump/pluto/graph"
	"github.com/pschlump/pluto/maxflow"
)

// maxflowMatching is the algs4 route to the same answer: build the
// unit-capacity flow network s -> side A -> side B -> t and run
// FordFulkerson; the maxflow value is the matching cardinality.  It is
// the benchmark reference for the "compose maxflow or augment directly"
// decision (backlog item 13) — direct integer augmentation wins by a
// large margin (see README.adoc).
func maxflowMatching(g *graph.Graph, sideA int) int {
	n := g.V()
	fg := maxflow.NewFlowNetwork(n + 2)
	s, t := n, n+1
	for v := range sideA {
		fg.AddEdge(s, v, 1)
	}
	for v := sideA; v < n; v++ {
		fg.AddEdge(v, t, 1)
	}
	for v := range sideA {
		for w := range g.Adj(v) { // w is on side B: v < sideA <= w
			fg.AddEdge(v, w, 1)
		}
	}
	return int(maxflow.NewFordFulkerson(fg, s, t).Value())
}

// benchGraph builds a fixed (PCG 42,7) random bipartite graph, side A
// first, so every algorithm is measured on the identical instance.
func benchGraph(sideA, sideB int, p float64) *graph.Graph {
	rng := rand.New(rand.NewPCG(42, 7))
	g := graph.NewGraph(sideA + sideB)
	for v := range sideA {
		for w := sideA; w < sideA+sideB; w++ {
			if rng.Float64() < p {
				g.AddEdge(v, w)
			}
		}
	}
	return g
}

// BenchmarkBipartite measures the two direct implementations against
// each other and against the maxflow composition, over sparse, dense
// and rectangular shapes.
func BenchmarkBipartite(b *testing.B) {
	for _, s := range []struct {
		name         string
		sideA, sideB int
		p            float64
	}{
		{"500x500-sparse", 500, 500, 0.02},
		{"500x500-dense", 500, 500, 0.10},
		{"1000x500", 1000, 500, 0.05},
	} {
		g := benchGraph(s.sideA, s.sideB, s.p)
		want := NewBipartiteMatching(g).Cardinality()
		if got := NewHopcroftKarp(g).Cardinality(); got != want {
			b.Fatalf("%s: HopcroftKarp %d != BipartiteMatching %d", s.name, got, want)
		}
		if got := maxflowMatching(g, s.sideA); got != want {
			b.Fatalf("%s: maxflow %d != BipartiteMatching %d", s.name, got, want)
		}
		b.Run(s.name+"/kuhn", func(b *testing.B) {
			for range b.N {
				NewBipartiteMatching(g)
			}
		})
		b.Run(s.name+"/hopcroft-karp", func(b *testing.B) {
			for range b.N {
				NewHopcroftKarp(g)
			}
		})
		b.Run(s.name+"/maxflow", func(b *testing.B) {
			for range b.N {
				maxflowMatching(g, s.sideA)
			}
		})
	}
}
