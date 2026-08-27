/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package graph_test

import (
	"fmt"

	"github.com/pschlump/pluto/graph"
)

// The 6-vertex, 8-edge undirected graph of Sedgwick's §4.1 trace figures.
// Adj yields neighbors in insertion order.
func Example() {
	g := graph.NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}

	fmt.Println(g.V(), "vertices,", g.E(), "edges")
	for v := 0; v < g.V(); v++ {
		fmt.Printf("%d:", v)
		for w := range g.Adj(v) {
			fmt.Printf(" %d", w)
		}
		fmt.Println()
	}
	// Output:
	// 6 vertices, 8 edges
	// 0: 5 1 2
	// 1: 2 0
	// 2: 4 3 1 0
	// 3: 2 4 5
	// 4: 2 3
	// 5: 0 3
}

// Depth-first search finds *a* path from the source to every reachable
// vertex — not necessarily a shortest one.
func ExampleNewDFSPaths() {
	g := graph.NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}

	p := graph.NewDFSPaths(g, 0)
	if path, ok := p.PathTo(4); ok {
		fmt.Println("path 0 to 4:", path)
	}
	// Output:
	// path 0 to 4: [0 5 3 2 4]
}

// Breadth-first search finds a *shortest* path (fewest edges) from the
// source — here 0-2-4, two edges, where DFS wanders 0-5-3-2-4.
func ExampleNewBFSPaths() {
	g := graph.NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}

	p := graph.NewBFSPaths(g, 0)
	for v := 0; v < g.V(); v++ {
		d, _ := p.DistTo(v)
		path, _ := p.PathTo(v)
		fmt.Printf("0 to %d (dist %d): %v\n", v, d, path)
	}
	// Output:
	// 0 to 0 (dist 0): [0]
	// 0 to 1 (dist 1): [0 1]
	// 0 to 2 (dist 1): [0 2]
	// 0 to 3 (dist 2): [0 5 3]
	// 0 to 4 (dist 2): [0 2 4]
	// 0 to 5 (dist 1): [0 5]
}

// Connected components on a graph with four components: {0,1,2}, {3,4},
// {5}, {6,7}.
func ExampleNewCC() {
	g := graph.NewGraph(8)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {3, 4}, {6, 7}} {
		g.AddEdge(e[0], e[1])
	}

	c := graph.NewCC(g)
	fmt.Println(c.Count(), "components")
	for id := 0; id < c.Count(); id++ {
		fmt.Printf("component %d:", id)
		for v := 0; v < g.V(); v++ {
			if vid, _ := c.ID(v); vid == id {
				fmt.Printf(" %d", v)
			}
		}
		fmt.Println()
	}
	fmt.Println("0 connected to 2:", c.Connected(0, 2))
	fmt.Println("0 connected to 3:", c.Connected(0, 3))
	// Output:
	// 4 components
	// component 0: 0 1 2
	// component 1: 3 4
	// component 2: 5
	// component 3: 6 7
	// 0 connected to 2: true
	// 0 connected to 3: false
}
