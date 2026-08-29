/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package bellman_ford_test

import (
	"fmt"

	"github.com/pschlump/pluto/bellman_ford"
	"github.com/pschlump/pluto/dijkstra"
)

// Shortest paths from vertex 0 in a small edge-weighted digraph with a
// negative edge — the case Dijkstra's algorithm cannot handle.  The
// negative edge 1->2 makes the route through vertex 1 the cheap one.
func Example() {
	g := dijkstra.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 6})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: -4})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 4, Weight: 2})

	sp := bellman_ford.NewBellmanFordSP(g, 0)
	fmt.Println("negative cycle:", sp.HasNegativeCycle())
	for v := 0; v < g.V(); v++ {
		fmt.Printf("0 -> %d: %g\n", v, sp.DistTo(v))
	}

	fmt.Print("path 0 -> 4:")
	for _, e := range sp.PathTo(4) {
		fmt.Printf(" %d->%d(%g)", e.From, e.To, e.Weight)
	}
	fmt.Println()
	// Output:
	// negative cycle: false
	// 0 -> 0: 0
	// 0 -> 1: 2
	// 0 -> 2: -2
	// 0 -> 3: -1
	// 0 -> 4: 1
	// path 0 -> 4: 0->1(2) 1->2(-4) 2->3(1) 3->4(2)
}

// When a negative cycle is reachable from the source, the shortest-path
// questions are undefined; HasNegativeCycle reports it and NegativeCycle
// exhibits one.
func ExampleBellmanFordSP_NegativeCycle() {
	g := dijkstra.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 1, Weight: -4}) // cycle 1->2->3->1, weight -2

	sp := bellman_ford.NewBellmanFordSP(g, 0)
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

// Shortest paths in a DAG by relaxing in topological order — linear
// time, negative weights allowed.
func ExampleNewAcyclicSP() {
	g := dijkstra.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 5})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: -3})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 3, Weight: 4})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 4, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 4, Weight: 20})

	sp := bellman_ford.NewAcyclicSP(g, 0)
	for v := 0; v < g.V(); v++ {
		fmt.Printf("0 -> %d: %g\n", v, sp.DistTo(v))
	}
	// Output:
	// 0 -> 0: 0
	// 0 -> 1: 2
	// 0 -> 2: -1
	// 0 -> 3: 0
	// 0 -> 4: 2
}

// Longest paths in the same DAG — the critical-path computation.
func ExampleNewAcyclicLP() {
	g := dijkstra.NewEdgeWeightedDigraph(5)
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 1, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 2, Weight: 5})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 2, Weight: -3})
	g.AddEdge(dijkstra.DirectedEdge{From: 1, To: 3, Weight: 4})
	g.AddEdge(dijkstra.DirectedEdge{From: 2, To: 3, Weight: 1})
	g.AddEdge(dijkstra.DirectedEdge{From: 3, To: 4, Weight: 2})
	g.AddEdge(dijkstra.DirectedEdge{From: 0, To: 4, Weight: 20})

	lp := bellman_ford.NewAcyclicLP(g, 0)
	for v := 0; v < g.V(); v++ {
		fmt.Printf("0 -> %d: %g\n", v, lp.DistTo(v))
	}
	// Output:
	// 0 -> 0: 0
	// 0 -> 1: 2
	// 0 -> 2: 5
	// 0 -> 3: 6
	// 0 -> 4: 20
}
