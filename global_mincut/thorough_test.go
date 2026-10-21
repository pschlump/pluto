package global_mincut

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"

	"github.com/pschlump/pluto/dijkstra"
	"github.com/pschlump/pluto/maxflow"
)

// maxflowMinCutBench is the benchmark twin of maxflowMinCut from the
// test file (no *testing.T in the b.N loop).
func maxflowMinCutBench(g *dijkstra.EdgeWeightedGraph) float64 {
	n := g.V()
	best := math.Inf(1)
	for t := 1; t < n; t++ {
		net := maxflow.NewFlowNetwork(n)
		for v := 0; v < n; v++ {
			for _, e := range g.Adj(v) {
				if u := e.Other(v); u != v && u > v {
					net.AddEdge(v, u, e.Weight)
					net.AddEdge(u, v, e.Weight)
				}
			}
		}
		if val := maxflow.NewFordFulkerson(net, 0, t).Value(); val < best {
			best = val
		}
	}
	return best
}
