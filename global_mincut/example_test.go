package global_mincut_test

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"fmt"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/global_mincut"
)

// Two triangles of weight-10 edges joined by a weight-1 bridge: the
// minimum cut severs just the bridge.
func ExampleNewGlobalMincut() {
	g := dijkstra.NewEdgeWeightedGraph(6)
	for _, e := range []dijkstra.Edge{
		{V: 0, W: 1, Weight: 10}, {V: 1, W: 2, Weight: 10}, {V: 2, W: 0, Weight: 10},
		{V: 3, W: 4, Weight: 10}, {V: 4, W: 5, Weight: 10}, {V: 5, W: 3, Weight: 10},
		{V: 2, W: 3, Weight: 1},
	} {
		g.AddEdge(e)
	}

	c := global_mincut.NewGlobalMincut(g)
	fmt.Println("min cut weight:", c.Weight())
	fmt.Println("cut side:", c.Cut())
	fmt.Println("vertex 0 in cut:", c.InCut(0))
	fmt.Println("vertex 3 in cut:", c.InCut(3))
	// Output:
	// min cut weight: 1
	// cut side: [3 4 5]
	// vertex 0 in cut: false
	// vertex 3 in cut: true
}
