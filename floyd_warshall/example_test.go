/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package floyd_warshall_test

import (
	"fmt"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/floyd_warshall"
)

// All-pairs shortest paths in a small digraph with negative edges (and
// no negative cycle) — the case Dijkstra's all-pairs search cannot
// handle.  The negative edge 2->0 makes the round trip through 2 the
// cheap way back from 3.
func Example() {
	g := dijkstra.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 3})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 1, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 3, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 0, Weight: -1}) // negative weights are legal

	fw := floyd_warshall.NewFloydWarshall(g)
	fmt.Println("negative cycle:", fw.HasNegativeCycle())
	for v := range g.V() {
		fmt.Printf("0 -> %d: %g\n", v, fw.Dist(0, v))
	}

	fmt.Print("path 0 -> 3:")
	for _, e := range fw.Path(0, 3) {
		fmt.Printf(" %d->%d(%g)", e.From, e.To, e.Weight)
	}
	fmt.Println()
	// Output:
	// negative cycle: false
	// 0 -> 0: 0
	// 0 -> 1: 2
	// 0 -> 2: 1
	// 0 -> 3: 4
	// path 0 -> 3: 0->2(1) 2->1(1) 1->3(2)
}

// When the graph contains a negative cycle, shortest paths are
// undefined: HasNegativeCycle reports it and NegativeCycle exhibits
// one — here a cycle that no single-source search from vertex 0 could
// ever see, because nothing reaches it from 0.
func ExampleFloydWarshall_NegativeCycle() {
	g := dijkstra.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 4, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 4, To: 2, Weight: -3}) // cycle 2->3->4->2, weight -1

	fw := floyd_warshall.NewFloydWarshall(g)
	fmt.Println("negative cycle:", fw.HasNegativeCycle())
	fmt.Print("the cycle:")
	for _, e := range fw.NegativeCycle() {
		fmt.Printf(" %d->%d(%g)", e.From, e.To, e.Weight)
	}
	fmt.Println()
	// Output:
	// negative cycle: true
	// the cycle: 4->2(-3) 2->3(1) 3->4(1)
}
