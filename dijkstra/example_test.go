/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package dijkstra_test

import (
	"fmt"

	"github.com/pschlump/pluto/dijkstra"
)

// Shortest paths from vertex 0 in a small edge-weighted digraph:
// distances from the source, and a reconstructed path.
func Example() {
	g := dijkstra.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 5})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 3, Weight: 6})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 3})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 4, Weight: 1})

	sp := dijkstra.NewDijkstraSP(g, 0)
	for v := 0; v < g.V(); v++ {
		fmt.Printf("0 -> %d: %g\n", v, sp.DistTo(v))
	}

	fmt.Print("path 0 -> 4:")
	for _, e := range sp.PathTo(4) {
		fmt.Printf(" %d->%d(%g)", e.From, e.To, e.Weight)
	}
	fmt.Println()
	// Output:
	// 0 -> 0: 0
	// 0 -> 1: 2
	// 0 -> 2: 3
	// 0 -> 3: 6
	// 0 -> 4: 7
	// path 0 -> 4: 0->1(2) 1->2(1) 2->3(3) 3->4(1)
}

// Unreachable vertices report +Inf, false and nil — never a panic.
func ExampleDijkstraSP_DistTo() {
	g := dijkstra.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 4})

	sp := dijkstra.NewDijkstraSP(g, 0)
	fmt.Println(sp.HasPathTo(1), sp.DistTo(1))
	fmt.Println(sp.HasPathTo(2), sp.DistTo(2))
	// Output:
	// true 4
	// false +Inf
}

// The undirected twin: each edge relaxes in both directions.
func ExampleNewDijkstraUndirectedSP() {
	g := dijkstra.NewEdgeWeightedGraph(4)
	g.AddEdge(dijkstra.Edge{V: 0, W: 1, Weight: 7})
	g.AddEdge(dijkstra.Edge{V: 1, W: 2, Weight: 2})
	g.AddEdge(dijkstra.Edge{V: 2, W: 3, Weight: 2})
	g.AddEdge(dijkstra.Edge{V: 0, W: 3, Weight: 20})

	sp := dijkstra.NewDijkstraUndirectedSP(g, 3)
	fmt.Printf("3 -> 0: %g\n", sp.DistTo(0))
	for _, e := range sp.PathTo(0) {
		fmt.Printf("%d--%d(%g)\n", e.V, e.W, e.Weight)
	}
	// Output:
	// 3 -> 0: 11
	// 2--3(2)
	// 1--2(2)
	// 0--1(7)
}

// One query object answers shortest paths between every pair of
// vertices.
func ExampleNewDijkstraAllPairsSP() {
	g := dijkstra.NewEdgeWeightedDigraph(3)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 10})

	ap := dijkstra.NewDijkstraAllPairsSP(g)
	for s := 0; s < g.V(); s++ {
		for t := 0; t < g.V(); t++ {
			if t > 0 {
				fmt.Print("  ")
			}
			fmt.Printf("%d->%d %g", s, t, ap.Dist(s, t))
		}
		fmt.Println()
	}
	// Output:
	// 0->0 0  0->1 1  0->2 3
	// 1->0 +Inf  1->1 0  1->2 2
	// 2->0 +Inf  2->1 +Inf  2->2 0
}
