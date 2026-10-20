/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// The examples in this file are compiled and run by `go test` (their
// output is checked against the // Output: comments) and appear on
// pkg.go.dev as the package documentation examples.
package floyd_warshall_ts_test

import (
	"fmt"

	"github.com/pschlump/pluto/dijkstra_ts"
	"github.com/pschlump/pluto/floyd_warshall_ts"
)

// All-pairs shortest paths with negative edges over a mutex-guarded
// graph: the constructor snapshots the adjacency under the read lock
// and computes lock-free; the query object is then safe to share.
func Example() {
	g := dijkstra_ts.NewEdgeWeightedDigraph(4)
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 1, Weight: 3})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 0, To: 2, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 1, Weight: 1})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 1, To: 3, Weight: 2})
	g.AddEdge(dijkstra_ts.DirectedEdge{From: 2, To: 0, Weight: -1}) // negative weights are legal

	fw := floyd_warshall_ts.NewFloydWarshall(g)
	fmt.Println("negative cycle:", fw.HasNegativeCycle())
	fmt.Println("0 -> 1:", fw.Dist(0, 1))
	fmt.Println("0 -> 3:", fw.Dist(0, 3))
	// Output:
	// negative cycle: false
	// 0 -> 1: 2
	// 0 -> 3: 4
}
