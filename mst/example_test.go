/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package mst_test

import (
	"fmt"
	"slices"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/mst"
)

// byEdge orders edges by weight, then by endpoints, so example output
// is deterministic.
func byEdge(a, b dijkstra.Edge) int {
	if a.Weight != b.Weight {
		if a.Weight < b.Weight {
			return -1
		}
		return 1
	}
	if a.V != b.V {
		return a.V - b.V
	}
	return a.W - b.W
}

// Kruskal's algorithm on a small hand-built graph: the MST takes the
// three cheap edges and skips the expensive 0--3 that would close a
// cycle.
func ExampleNewKruskalMST() {
	g := dijkstra.NewEdgeWeightedGraph(4)
	g.AddEdge(dijkstra.Edge{V: 0, W: 1, Weight: 1})
	g.AddEdge(dijkstra.Edge{V: 1, W: 2, Weight: 2})
	g.AddEdge(dijkstra.Edge{V: 2, W: 3, Weight: 3})
	g.AddEdge(dijkstra.Edge{V: 0, W: 3, Weight: 10})

	mst := mst.NewKruskalMST(g)
	edges := mst.Edges()
	slices.SortFunc(edges, byEdge)
	for _, e := range edges {
		fmt.Printf("%d--%d (%g)\n", e.V, e.W, e.Weight)
	}
	fmt.Printf("total weight %g\n", mst.Weight())
	// Output:
	// 0--1 (1)
	// 1--2 (2)
	// 2--3 (3)
	// total weight 6
}

// The eager version of Prim's algorithm answers the same queries on the
// same graph.
func ExampleNewPrimMST() {
	g := dijkstra.NewEdgeWeightedGraph(4)
	g.AddEdge(dijkstra.Edge{V: 0, W: 1, Weight: 1})
	g.AddEdge(dijkstra.Edge{V: 1, W: 2, Weight: 2})
	g.AddEdge(dijkstra.Edge{V: 2, W: 3, Weight: 3})
	g.AddEdge(dijkstra.Edge{V: 0, W: 3, Weight: 10})

	mst := mst.NewPrimMST(g)
	fmt.Printf("%d edges, total weight %g\n", mst.Len(), mst.Weight())
	// Output:
	// 3 edges, total weight 6
}

// On a disconnected graph every constructor produces the minimum
// spanning FOREST — one tree per component, so fewer than V-1 edges.
func ExampleNewLazyPrimMST() {
	g := dijkstra.NewEdgeWeightedGraph(5)
	g.AddEdge(dijkstra.Edge{V: 0, W: 1, Weight: 4})
	g.AddEdge(dijkstra.Edge{V: 1, W: 2, Weight: 5})
	g.AddEdge(dijkstra.Edge{V: 3, W: 4, Weight: 6})

	mst := mst.NewLazyPrimMST(g) // components {0,1,2} and {3,4}
	edges := mst.Edges()
	slices.SortFunc(edges, byEdge)
	for _, e := range edges {
		fmt.Printf("%d--%d (%g)\n", e.V, e.W, e.Weight)
	}
	fmt.Printf("%d edges for %d vertices, total weight %g\n", mst.Len(), g.V(), mst.Weight())
	// Output:
	// 0--1 (4)
	// 1--2 (5)
	// 3--4 (6)
	// 3 edges for 5 vertices, total weight 15
}
