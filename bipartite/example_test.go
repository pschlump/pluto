/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bipartite_test

import (
	"fmt"

	"github.com/pschlump/pluto/bipartite"
	"github.com/pschlump/pluto/graph"
)

func ExampleNewBipartiteMatching() {
	// Side A = {0,1,2}, side B = {3,4,5}: three tasks, three workers.
	g := graph.NewGraph(6)
	for _, e := range [][2]int{{0, 3}, {0, 4}, {1, 3}, {1, 5}, {2, 4}} {
		g.AddEdge(e[0], e[1])
	}

	bm := bipartite.NewBipartiteMatching(g)
	fmt.Println("cardinality:", bm.Cardinality())
	for _, e := range bm.Edges() {
		fmt.Printf("matched %d-%d\n", e[0], e[1])
	}
	fmt.Println("minimum vertex cover:", bm.MinVertexCover())
	fmt.Println("matched:", bm.InMatching(2), "mate of 0:", bm.Mate(0))

	// Output:
	// cardinality: 3
	// matched 0-3
	// matched 1-5
	// matched 2-4
	// minimum vertex cover: [0 1 2]
	// matched: true mate of 0: 3
}

func ExampleNewHopcroftKarp() {
	// The same graph through the phased algorithm; on this instance the
	// two find the same maximum matching (in general they may differ —
	// both are maximum, and the cardinality and cover agree).
	g := graph.NewGraph(6)
	for _, e := range [][2]int{{0, 3}, {0, 4}, {1, 3}, {1, 5}, {2, 4}} {
		g.AddEdge(e[0], e[1])
	}

	hk := bipartite.NewHopcroftKarp(g)
	fmt.Println("cardinality:", hk.Cardinality())
	for _, e := range hk.Edges() {
		fmt.Printf("matched %d-%d\n", e[0], e[1])
	}
	fmt.Println("minimum vertex cover:", hk.MinVertexCover())

	// Output:
	// cardinality: 3
	// matched 0-3
	// matched 1-5
	// matched 2-4
	// minimum vertex cover: [0 1 2]
}
