/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package maxflow_test

import (
	"fmt"

	"github.com/pschlump/pluto/maxflow"
)

// Ford-Fulkerson (Edmonds-Karp) on a small hand-built network: both
// edges out of the source saturate, so the maxflow equals the source
// cut capacity 2+3 = 5 and the source side of the min cut is {0}.
func ExampleNewFordFulkerson() {
	g := maxflow.NewFlowNetwork(4)
	g.AddEdge(0, 1, 2)
	g.AddEdge(0, 2, 3)
	g.AddEdge(1, 2, 1)
	g.AddEdge(1, 3, 1)
	g.AddEdge(2, 3, 4)

	ff := maxflow.NewFordFulkerson(g, 0, 3)
	fmt.Printf("maxflow from %d to %d: %g\n", ff.S(), ff.T(), ff.Value())
	var side []int
	for v := 0; v < g.V(); v++ {
		if ff.InMinCut(v) {
			side = append(side, v)
		}
	}
	fmt.Println("source side of the min cut:", side)
	// Output:
	// maxflow from 0 to 3: 5
	// source side of the min cut: [0]
}

// Edges reports the computed flow on every edge; the caller's network
// is never mutated.
func ExampleFordFulkerson_Edges() {
	g := maxflow.NewFlowNetwork(4)
	g.AddEdge(0, 1, 2)
	g.AddEdge(0, 2, 3)
	g.AddEdge(1, 2, 1)
	g.AddEdge(1, 3, 1)
	g.AddEdge(2, 3, 4)

	ff := maxflow.NewFordFulkerson(g, 0, 3)
	for _, e := range ff.Edges() {
		fmt.Printf("%d->%d %g/%g\n", e.From, e.To, e.Flow, e.Capacity)
	}
	// Output:
	// 0->1 2/2
	// 0->2 3/3
	// 1->2 1/1
	// 1->3 1/1
	// 2->3 4/4
}
