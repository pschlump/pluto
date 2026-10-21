/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package digraph_test

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/digraph"
)

// A small digraph: 0->1, 0->2, 1->3, 2->3, 3->4.  Adj yields
// out-neighbors in insertion order.
func Example() {
	g := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	fmt.Println(g.V(), "vertices,", g.E(), "edges")
	for v := 0; v < g.V(); v++ {
		out, _ := g.OutDegree(v)
		in, _ := g.InDegree(v)
		fmt.Printf("%d (out %d, in %d):", v, out, in)
		for w := range g.Adj(v) {
			fmt.Printf(" %d", w)
		}
		fmt.Println()
	}
	// Output:
	// 5 vertices, 5 edges
	// 0 (out 2, in 0): 1 2
	// 1 (out 1, in 1): 3
	// 2 (out 1, in 1): 3
	// 3 (out 1, in 2): 4
	// 4 (out 0, in 1):
}

// Multi-source reachability: the vertices reachable from 0 OR 4.
func ExampleNewDirectedDFS() {
	g := digraph.NewDigraph(6)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {4, 5}} {
		g.AddEdge(e[0], e[1])
	}

	d := digraph.NewDirectedDFS(g, 0, 4)
	fmt.Print("reachable:")
	for v := 0; v < g.V(); v++ {
		if d.Marked(v) {
			fmt.Printf(" %d", v)
		}
	}
	fmt.Println()
	fmt.Println("count:", d.Count())
	// Output:
	// reachable: 0 1 2 3 4 5
	// count: 6
}

// Breadth-first search finds a shortest directed path (fewest edges) from
// the source.
func ExampleNewBFSDirectedPaths() {
	g := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	p := digraph.NewBFSDirectedPaths(g, 0)
	for v := 0; v < g.V(); v++ {
		d, _ := p.DistTo(v)
		path, _ := p.PathTo(v)
		fmt.Printf("0 to %d (dist %d): %v\n", v, d, path)
	}
	// Output:
	// 0 to 0 (dist 0): [0]
	// 0 to 1 (dist 1): [0 1]
	// 0 to 2 (dist 1): [0 2]
	// 0 to 3 (dist 2): [0 1 3]
	// 0 to 4 (dist 3): [0 1 3 4]
}

// Directed cycle detection: the cycle 0->1->2->0, reported in edge order
// with the start vertex repeated at the end.
func ExampleNewDirectedCycle() {
	g := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	c := digraph.NewDirectedCycle(g)
	fmt.Println("has cycle:", c.HasCycle())
	fmt.Println("cycle:", c.Cycle())
	// Output:
	// has cycle: true
	// cycle: [0 1 2 0]
}

// Topological sort: for every edge v->w, v comes before w.  A digraph
// with a directed cycle has no topological order.
func ExampleNewTopological() {
	dag := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {1, 3}, {2, 3}, {3, 4}} {
		dag.AddEdge(e[0], e[1])
	}
	top := digraph.NewTopological(dag)
	fmt.Println("DAG order:", top.Order())

	cyclic := digraph.NewDigraph(3)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}} {
		cyclic.AddEdge(e[0], e[1])
	}
	fmt.Println("cyclic has order:", digraph.NewTopological(cyclic).HasOrder())
	// Output:
	// DAG order: [0 2 1 3 4]
	// cyclic has order: false
}

// Kosaraju's algorithm finds the strong components: maximal sets of
// vertices that are mutually reachable.
func ExampleNewKosarajuSCC() {
	g := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	scc := digraph.NewKosarajuSCC(g)
	fmt.Println(scc.Count(), "strong components")
	for id := 0; id < scc.Count(); id++ {
		fmt.Printf("component %d:", id)
		for v := 0; v < g.V(); v++ {
			if vid, _ := scc.ID(v); vid == id {
				fmt.Printf(" %d", v)
			}
		}
		fmt.Println()
	}
	fmt.Println("0 strongly connected to 2:", scc.StronglyConnected(0, 2))
	fmt.Println("0 strongly connected to 3:", scc.StronglyConnected(0, 3))
	// Output:
	// 3 strong components
	// component 0: 4
	// component 1: 3
	// component 2: 0 1 2
	// 0 strongly connected to 2: true
	// 0 strongly connected to 3: false
}

// Transitive closure on a digraph with a 4-cycle {0,1,2,3} and a
// separate edge 4->5: every vertex of the cycle reaches every other
// vertex of the cycle, nothing reaches 4 or 5 from the cycle, and every
// vertex reaches itself.
func ExampleNewTransitiveClosure() {
	g := digraph.NewDigraph(6)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 3}, {3, 0}, {4, 5}} {
		g.AddEdge(e[0], e[1])
	}

	tc := digraph.NewTransitiveClosure(g)
	for _, q := range [][2]int{{0, 3}, {3, 0}, {2, 1}, {0, 4}, {4, 5}, {5, 4}, {5, 5}} {
		fmt.Printf("%d reaches %d: %v\n", q[0], q[1], tc.Reachable(q[0], q[1]))
	}
	// Output:
	// 0 reaches 3: true
	// 3 reaches 0: true
	// 2 reaches 1: true
	// 0 reaches 4: false
	// 4 reaches 5: true
	// 5 reaches 4: false
	// 5 reaches 5: true
}

// MarshalJSON encodes the digraph as a JSON object with the vertex count
// and the edge list, in natural iteration order.
func ExampleDigraph_MarshalJSON() {
	g := digraph.NewDigraph(4)
	for _, e := range [][2]int{{0, 1}, {0, 2}, {1, 3}} {
		g.AddEdge(e[0], e[1])
	}

	b, err := json.Marshal(g)
	fmt.Println(string(b), err)
	// Output:
	// {"vertices":4,"edges":[[0,1],[0,2],[1,3]]} <nil>
}

// UnmarshalJSON replaces the contents of the digraph from a JSON object;
// the vertex set becomes 0..vertices-1 and the edges are added in
// document order.
func ExampleDigraph_UnmarshalJSON() {
	g := digraph.NewDigraph(1)
	if err := json.Unmarshal([]byte(`{"vertices":3,"edges":[[0,1],[0,2],[1,2]]}`), g); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("vertices:", g.V(), "edges:", g.E())
	for v := 0; v < g.V(); v++ {
		for w := range g.Adj(v) {
			fmt.Println(v, "->", w)
		}
	}
	// Output:
	// vertices: 3 edges: 3
	// 0 -> 1
	// 0 -> 2
	// 1 -> 2
}

// Tarjan's single-pass SCC on the same digraph as the Kosaraju example
// (the two agree on the partition; the component ids come out in
// retreat order here, which happens to match).
func ExampleNewTarjanSCC() {
	g := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	scc := digraph.NewTarjanSCC(g)
	fmt.Println(scc.Count(), "strong components")
	for id := 0; id < scc.Count(); id++ {
		fmt.Printf("component %d:", id)
		for v := 0; v < g.V(); v++ {
			if vid, _ := scc.ID(v); vid == id {
				fmt.Printf(" %d", v)
			}
		}
		fmt.Println()
	}
	fmt.Println("0 strongly connected to 2:", scc.StronglyConnected(0, 2))
	fmt.Println("0 strongly connected to 3:", scc.StronglyConnected(0, 3))
	// Output:
	// 3 strong components
	// component 0: 4
	// component 1: 3
	// component 2: 0 1 2
	// 0 strongly connected to 2: true
	// 0 strongly connected to 3: false
}

// Gabow's path-based SCC on the same digraph.
func ExampleNewGabowSCC() {
	g := digraph.NewDigraph(5)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {2, 3}, {3, 4}} {
		g.AddEdge(e[0], e[1])
	}

	scc := digraph.NewGabowSCC(g)
	fmt.Println(scc.Count(), "strong components")
	for id := 0; id < scc.Count(); id++ {
		fmt.Printf("component %d:", id)
		for v := 0; v < g.V(); v++ {
			if vid, _ := scc.ID(v); vid == id {
				fmt.Printf(" %d", v)
			}
		}
		fmt.Println()
	}
	fmt.Println("0 strongly connected to 2:", scc.StronglyConnected(0, 2))
	fmt.Println("0 strongly connected to 3:", scc.StronglyConnected(0, 3))
	// Output:
	// 3 strong components
	// component 0: 4
	// component 1: 3
	// component 2: 0 1 2
	// 0 strongly connected to 2: true
	// 0 strongly connected to 3: false
}

// A directed Eulerian cycle: every vertex balanced, every edge used
// exactly once by a closed walk.
func ExampleNewDirectedEulerianCycle() {
	g := digraph.NewDigraph(4)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {0, 3}, {3, 0}} {
		g.AddEdge(e[0], e[1])
	}

	ec := digraph.NewDirectedEulerianCycle(g)
	fmt.Println("has cycle:", ec.HasEulerianCycle())
	fmt.Println("cycle:", ec.Cycle())
	// Output:
	// has cycle: true
	// cycle: [0 1 2 0 3 0]
}

// A directed Eulerian path: one start (out-in = 1), one end
// (in-out = 1), every other vertex balanced.
func ExampleNewDirectedEulerianPath() {
	g := digraph.NewDigraph(4)
	for _, e := range [][2]int{{0, 1}, {1, 2}, {2, 0}, {0, 3}} {
		g.AddEdge(e[0], e[1])
	}

	ep := digraph.NewDirectedEulerianPath(g)
	fmt.Println("has path:", ep.HasEulerianPath())
	fmt.Println("path:", ep.Path())
	// Output:
	// has path: true
	// path: [0 1 2 0 3]
}
