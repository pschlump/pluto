/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package assignment_problem

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/dijkstra"
)

// composedDijkstra solves an instance the way algs4's
// AssignmentProblem does: build the residual alternating-path graph as
// an explicit EdgeWeightedDigraph every round (source to free rows at
// 0, forward edges at reduced cost, matched edges reversed at 0, free
// columns to the sink at their column dual) and run dijkstra.DijkstraSP
// over it — O(n³ log n) with n² edge allocations per round.  It is the
// benchmark reference for the "compose or scan directly" decision
// (backlog item 14), the bipartite package's maxflowMatching precedent.
func composedDijkstra(w [][]float64) float64 {
	n := len(w)
	minW := 0.0
	for i := range n {
		for j := range n {
			if w[i][j] < minW {
				minW = w[i][j]
			}
		}
	}
	px := make([]float64, n)
	py := make([]float64, n)
	xy := make([]int, n)
	yx := make([]int, n)
	for i := range n {
		xy[i], yx[i] = unmatched, unmatched
	}
	s, t := 2*n, 2*n+1
	for range n {
		g := dijkstra.NewEdgeWeightedDigraph(2*n + 2)
		for i := range n {
			if xy[i] == unmatched {
				g.AddEdge(dijkstra.DirectedEdge{From: s, To: i})
			}
		}
		for j := range n {
			if yx[j] == unmatched {
				g.AddEdge(dijkstra.DirectedEdge{From: n + j, To: t, Weight: py[j]})
			}
		}
		for i := range n {
			for j := range n {
				if xy[i] == j {
					g.AddEdge(dijkstra.DirectedEdge{From: n + j, To: i})
				} else {
					g.AddEdge(dijkstra.DirectedEdge{From: i, To: n + j, Weight: w[i][j] - minW + px[i] - py[j]})
				}
			}
		}
		sp := dijkstra.NewDijkstraSP(g, s)
		for _, e := range sp.PathTo(t) {
			if e.From < n { // a forward edge on the augmenting path: pair its ends
				xy[e.From] = e.To - n
				yx[e.To-n] = e.From
			}
		}
		for i := range n {
			px[i] += sp.DistTo(i)
		}
		for j := range n {
			py[j] += sp.DistTo(n + j)
		}
	}
	var total float64
	for i := range n {
		total += w[i][xy[i]]
	}
	return total
}

// benchMatrix builds a fixed (PCG n,42) integer matrix — algs4's main
// draws 3-digit weights; so does this — so every route is measured on
// the identical instance.
func benchMatrix(n int) [][]float64 {
	rng := rand.New(rand.NewPCG(uint64(n), 42))
	w := make([][]float64, n)
	for i := range w {
		w[i] = make([]float64, n)
		for j := range n {
			w[i][j] = float64(100 + rng.IntN(900))
		}
	}
	return w
}

// BenchmarkSolve measures the shipped solver (dense in-matrix Dijkstra,
// O(n³)) against the algs4-style dijkstra composition (O(n³ log n)),
// first checking the two agree on the optimum.
func BenchmarkSolve(b *testing.B) {
	for _, n := range []int{100, 200, 400} {
		w := benchMatrix(n)
		want := NewAssignmentProblem(w).Weight()
		if got := composedDijkstra(w); got != want {
			b.Fatalf("n=%d: composed dijkstra %v != direct %v", n, got, want)
		}
		b.Run("n="+strconv.Itoa(n)+"/direct", func(b *testing.B) {
			for range b.N {
				NewAssignmentProblem(w)
			}
		})
		if n <= 200 { // the composition is minutes-slow far past this
			b.Run("n="+strconv.Itoa(n)+"/dijkstra-composed", func(b *testing.B) {
				for range b.N {
					composedDijkstra(w)
				}
			})
		}
	}
}
