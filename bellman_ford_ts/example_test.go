/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package bellman_ford_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/bellman_ford_ts"
	"github.com/pschlump/pluto/dijkstra_ts"
)

// Shortest paths from vertex 0 in a small edge-weighted digraph with a
// negative edge, computed on a mutex-guarded graph: the constructor
// snapshots the adjacency under the read lock, then searches lock-free.
func Example() {
	g := dijkstra_ts.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 2, Weight: 6})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 1, To: 2, Weight: -4})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 3, To: 4, Weight: 2})

	sp := bellman_ford_ts.NewBellmanFordSP(g, 0)
	fmt.Println("negative cycle:", sp.HasNegativeCycle())
	for v := 0; v < g.V(); v++ {
		fmt.Printf("0 -> %d: %g\n", v, sp.DistTo(v))
	}
	// Output:
	// negative cycle: false
	// 0 -> 0: 0
	// 0 -> 1: 2
	// 0 -> 2: -2
	// 0 -> 3: -1
	// 0 -> 4: 1
}

// When a negative cycle is reachable from the source, HasNegativeCycle
// reports it and NegativeCycle exhibits one.
func ExampleBellmanFordSP_NegativeCycle() {
	g := dijkstra_ts.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 1, To: 2, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 3, To: 1, Weight: -4}) // cycle 1->2->3->1, weight -2

	sp := bellman_ford_ts.NewBellmanFordSP(g, 0)
	fmt.Println("negative cycle:", sp.HasNegativeCycle())
	sum := 0.0
	for _, e := range sp.NegativeCycle() {
		sum += e.Weight
	}
	fmt.Println("cycle weight:", sum)
	// Output:
	// negative cycle: true
	// cycle weight: -2
}

// Shortest and longest paths in a DAG agree at the extremes of the same
// precedence structure.
func ExampleNewAcyclicLP() {
	g := dijkstra_ts.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 1, To: 2, Weight: -3})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 2, Weight: 5})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 3, Weight: 1})

	sp := bellman_ford_ts.NewAcyclicSP(g, 0)
	lp := bellman_ford_ts.NewAcyclicLP(g, 0)
	fmt.Printf("shortest 0 -> 3: %g\n", sp.DistTo(3))
	fmt.Printf("longest  0 -> 3: %g\n", lp.DistTo(3))
	// Output:
	// shortest 0 -> 3: 0
	// longest  0 -> 3: 6
}
