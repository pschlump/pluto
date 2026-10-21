/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package graph_ts_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/graph_ts"
)

// The 6-vertex, 8-edge undirected graph of Sedgwick's §4.1 trace figures.
// Adj yields neighbors in insertion order.
func Example() {
	g := graph_ts.NewGraph(6)
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
	g := graph_ts.NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}

	p := graph_ts.NewDFSPaths(g, 0)
	if path, ok := p.PathTo(4); ok {
		fmt.Println("path 0 to 4:", path)
	}
	// Output:
	// path 0 to 4: [0 5 3 2 4]
}

// Breadth-first search finds a *shortest* path (fewest edges) from the
// source — here 0-2-4, two edges, where DFS wanders 0-5-3-2-4.
func ExampleNewBFSPaths() {
	g := graph_ts.NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}

	p := graph_ts.NewBFSPaths(g, 0)
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
	g := graph_ts.NewGraph(8)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {3, 4}, {6, 7}} {
		g.AddEdge(e[0], e[1])
	}

	c := graph_ts.NewCC(g)
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

// Cycle detection on the tinyG trace graph: it is connected with
// E = 8 > V-1 = 5, so a cycle exists.
func ExampleNewCycle() {
	g := graph_ts.NewGraph(6)
	for _, e := range [][2]int{{0, 5}, {2, 4}, {2, 3}, {1, 2}, {0, 1}, {3, 4}, {3, 5}, {0, 2}} {
		g.AddEdge(e[0], e[1])
	}

	c := graph_ts.NewCycle(g)
	fmt.Println("has cycle:", c.HasCycle())
	fmt.Println("cycle:", c.Cycle())
	// Output:
	// has cycle: true
	// cycle: [3 2 4 3]
}

// Bipartiteness: a 6-cycle is 2-colorable (every edge crosses between
// the two color classes), a triangle is not.
func ExampleNewTwoColor() {
	even := graph_ts.NewGraph(6)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 4}, {4, 5}, {5, 0}} {
		even.AddEdge(e[0], e[1])
	}
	tc := graph_ts.NewTwoColor(even)
	fmt.Println("6-cycle bipartite:", tc.IsBipartite())
	for v := 0; v < 3; v++ {
		c, _ := tc.Color(v)
		fmt.Printf("vertex %d color %v\n", v, c)
	}

	odd := graph_ts.NewGraph(3)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}} {
		odd.AddEdge(e[0], e[1])
	}
	fmt.Println("triangle bipartite:", graph_ts.NewTwoColor(odd).IsBipartite())
	// Output:
	// 6-cycle bipartite: true
	// vertex 0 color false
	// vertex 1 color true
	// vertex 2 color false
	// triangle bipartite: false
}

// MarshalJSON encodes the graph as a JSON object with the vertex list
// and the edge list (each edge once, in canonical order).
func ExampleGraph_MarshalJSON() {
	g := graph_ts.NewGraph(3)
	g.AddEdge(0, 2)
	g.AddEdge(1, 2)

	b, err := json.Marshal(g)
	fmt.Println(string(b), err)
	// Output:
	// {"vertices":[0,1,2],"edges":[[0,2],[1,2]]} <nil>
}

// UnmarshalJSON replaces the contents of the graph from a JSON document,
// resizing the vertex set to the vertex list.
func ExampleGraph_UnmarshalJSON() {
	g := graph_ts.NewGraph(2)
	if err := json.Unmarshal([]byte(`{"vertices":[0,1,2,3],"edges":[[0,1],[1,2],[2,3]]}`), g); err != nil {
		fmt.Println("error:", err)
		return
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
	// 4 vertices, 3 edges
	// 0: 1
	// 1: 0 2
	// 2: 1 3
	// 3: 2
}

// An Eulerian cycle: every degree even and the edge-bearing vertices
// connected, so one closed walk crosses every edge exactly once (two
// triangles joined at vertex 1).
func ExampleNewEulerianCycle() {
	g := graph_ts.NewGraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {1, 3}, {3, 4}, {4, 1}} {
		g.AddEdge(e[0], e[1])
	}

	ec := graph_ts.NewEulerianCycle(g)
	fmt.Println("has cycle:", ec.HasEulerianCycle())
	fmt.Println("cycle:", ec.Cycle())
	// Output:
	// has cycle: true
	// cycle: [0 1 3 4 1 2 0]
}

// An Eulerian path: exactly two odd-degree vertices (0 and 3), so a
// walk crosses every edge exactly once from one odd vertex to the
// other.
func ExampleNewEulerianPath() {
	g := graph_ts.NewGraph(4)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}} {
		g.AddEdge(e[0], e[1])
	}

	ec := graph_ts.NewEulerianCycle(g)
	ep := graph_ts.NewEulerianPath(g)
	fmt.Println("has cycle:", ec.HasEulerianCycle())
	fmt.Println("has path:", ep.HasEulerianPath())
	fmt.Println("path:", ep.Path())
	// Output:
	// has cycle: false
	// has path: true
	// path: [0 1 2 3]
}
