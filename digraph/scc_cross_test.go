package digraph

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

// canonicalComponents renders an SCC result as the sorted set of sorted
// component vertex lists — the three algorithms assign component ids in
// different orders (Kosaraju by reverse-graph roots, Tarjan and Gabow by
// DFS retreat order), so the partition, not the numbering, is compared.
// id is the ID method value of the query object.
func canonicalComponents(id func(v int) (int, bool), n int) string {
	parts := make(map[int][]int)
	for v := 0; v < n; v++ {
		c, _ := id(v)
		parts[c] = append(parts[c], v)
	}
	var lists [][]int
	for _, p := range parts {
		slices.Sort(p)
		lists = append(lists, p)
	}
	slices.SortFunc(lists, func(a, b []int) int {
		return slices.Compare(a, b)
	})
	return fmt.Sprint(lists)
}

// TestSCCThreeWayAgreement cross-checks Kosaraju, Tarjan, and Gabow on
// randomized digraphs (self-loops and parallel edges included) — the
// three partitions must agree exactly.
func TestSCCThreeWayAgreement(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	for step := 0; step < 400; step++ {
		n := 1 + rng.Intn(24)
		g := NewDigraph(n)
		m := rng.Intn(3 * n)
		for i := 0; i < m; i++ {
			g.AddEdge(rng.Intn(n), rng.Intn(n))
		}
		kosaraju := NewKosarajuSCC(g)
		tarjan := NewTarjanSCC(g)
		gabow := NewGabowSCC(g)
		if kosaraju.Count() != tarjan.Count() || tarjan.Count() != gabow.Count() {
			t.Fatalf("step %d: component counts disagree: kosaraju=%d tarjan=%d gabow=%d",
				step, kosaraju.Count(), tarjan.Count(), gabow.Count())
		}
		for v := 0; v < n; v++ {
			for w := v + 1; w < n; w++ {
				k, ta, ga := kosaraju.StronglyConnected(v, w), tarjan.StronglyConnected(v, w), gabow.StronglyConnected(v, w)
				if k != ta || ta != ga {
					t.Fatalf("step %d: StronglyConnected(%d,%d) disagrees: %v/%v/%v", step, v, w, k, ta, ga)
				}
			}
		}
		want := canonicalComponents(kosaraju.ID, n)
		if got := canonicalComponents(tarjan.ID, n); got != want {
			t.Fatalf("step %d: Tarjan partition %s != Kosaraju partition %s", step, got, want)
		}
		if got := canonicalComponents(gabow.ID, n); got != want {
			t.Fatalf("step %d: Gabow partition %s != Kosaraju partition %s", step, got, want)
		}
	}
}

func BenchmarkKosarajuSCC(b *testing.B) { benchmarkSCC(b, NewKosarajuSCC) }

func BenchmarkTarjanSCC(b *testing.B) { benchmarkSCC(b, NewTarjanSCC) }

func BenchmarkGabowSCC(b *testing.B) { benchmarkSCC(b, NewGabowSCC) }

// benchmarkSCC times one SCC construction on a fixed-seed random
// digraph (n=1000, m=5000).  All three share the b.N loop shape.
func benchmarkSCC[T interface{ Count() int }](b *testing.B, build func(*Digraph) T) {
	b.StopTimer()
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	g := NewDigraph(1000)
	for i := 0; i < 5000; i++ {
		g.AddEdge(rng.Intn(1000), rng.Intn(1000))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if s := build(g); s.Count() == 0 {
			b.Fatal("unexpected empty result")
		}
	}
}
