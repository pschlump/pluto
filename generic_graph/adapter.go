/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import (
	"fmt"
	"math"

	"github.com/pschlump/pluto/digraph"
	"github.com/pschlump/pluto/dijkstra"
	intgraph "github.com/pschlump/pluto/graph"
)

// DenseAdapter assigns temporary contiguous integer IDs and builds independent
// classic graph/digraph snapshots. Empty graphs have nil IntGraph/IntDigraph
// because those classic constructors require at least one vertex.
type DenseAdapter[N any] struct {
	nodes   []N
	index   *indexedSet[N]
	graph   *intgraph.Graph
	digraph *digraph.Digraph
}

// Dense builds an adapter in the graph's configured node iteration order.
func (g *Graph[N]) Dense() *DenseAdapter[N] {
	if g == nil {
		return nil
	}
	result := &DenseAdapter[N]{nodes: g.NodesSlice()}
	result.index = newIndexedSet(g.config.Equal, g.config.Hash, len(result.nodes), g.config.native)
	for _, node := range result.nodes {
		result.index.add(node)
	}
	if len(result.nodes) == 0 {
		return result
	}
	if g.config.Directed {
		result.digraph = digraph.NewDigraph(len(result.nodes))
	} else {
		result.graph = intgraph.NewGraph(len(result.nodes))
	}
	for pair := range g.Edges() {
		u, _ := result.index.lookup(pair.Source)
		v, _ := result.index.lookup(pair.Target)
		if g.config.Directed {
			result.digraph.AddEdge(u, v)
		} else {
			result.graph.AddEdge(u, v)
		}
	}
	return result
}

func (a *DenseAdapter[N]) Len() int {
	if a == nil {
		return 0
	}
	return len(a.nodes)
}
func (a *DenseAdapter[N]) NodeID(node N) (int, bool) { return a.index.lookup(node) }
func (a *DenseAdapter[N]) Node(id int) (N, bool) {
	if a == nil || id < 0 || id >= len(a.nodes) {
		var zero N
		return zero, false
	}
	return a.nodes[id], true
}
func (a *DenseAdapter[N]) Nodes() []N {
	if a == nil {
		return nil
	}
	return append([]N(nil), a.nodes...)
}
func (a *DenseAdapter[N]) IntGraph() *intgraph.Graph {
	if a == nil {
		return nil
	}
	return a.graph
}
func (a *DenseAdapter[N]) IntDigraph() *digraph.Digraph {
	if a == nil {
		return nil
	}
	return a.digraph
}

// ShortestPaths maps a Dijkstra result back to generic nodes.
type ShortestPaths[N any] struct {
	nodes     []N
	index     *indexedSet[N]
	distances []float64
	paths     [][]int
	source    int
}

// Dijkstra computes nonnegative float64-valued shortest paths through an
// explicit dense adapter. Unknown sources, empty graphs, and negative weights
// return errors rather than classic-package panics.
func Dijkstra[N any](graph *ValueGraph[N, float64], source N) (*ShortestPaths[N], error) {
	if graph == nil || graph.NodeCount() == 0 {
		return nil, fmt.Errorf("%w: empty graph", ErrUnknownNode)
	}
	if !graph.ContainsNode(source) {
		return nil, ErrUnknownNode
	}
	result := &ShortestPaths[N]{nodes: graph.NodesSlice()}
	result.index = newIndexedSet(graph.graph.config.Equal, graph.graph.config.Hash, len(result.nodes), graph.graph.config.native)
	for _, node := range result.nodes {
		result.index.add(node)
	}
	result.source, _ = result.index.lookup(source)
	result.distances = make([]float64, len(result.nodes))
	result.paths = make([][]int, len(result.nodes))
	if graph.IsDirected() {
		weighted := dijkstra.NewEdgeWeightedDigraph(len(result.nodes))
		for pair := range graph.Edges() {
			weight, _ := graph.EdgeValue(pair.Source, pair.Target)
			if weight < 0 || math.IsNaN(weight) {
				return nil, fmt.Errorf("generic_graph: negative or NaN edge weight")
			}
			u, _ := result.index.lookup(pair.Source)
			v, _ := result.index.lookup(pair.Target)
			weighted.AddEdge(dijkstra.DirectedEdge{From: u, To: v, Weight: weight})
		}
		paths := dijkstra.NewDijkstraSP(weighted, result.source)
		for id := range result.nodes {
			result.distances[id] = paths.DistTo(id)
			for _, edge := range paths.PathTo(id) {
				result.paths[id] = append(result.paths[id], edge.To)
			}
		}
		return result, nil
	}
	weighted := dijkstra.NewEdgeWeightedGraph(len(result.nodes))
	for pair := range graph.Edges() {
		weight, _ := graph.EdgeValue(pair.Source, pair.Target)
		if weight < 0 || math.IsNaN(weight) {
			return nil, fmt.Errorf("generic_graph: negative or NaN edge weight")
		}
		u, _ := result.index.lookup(pair.Source)
		v, _ := result.index.lookup(pair.Target)
		weighted.AddEdge(dijkstra.Edge{V: u, W: v, Weight: weight})
	}
	paths := dijkstra.NewDijkstraUndirectedSP(weighted, result.source)
	for id := range result.nodes {
		result.distances[id] = paths.DistTo(id)
		current := result.source
		for _, edge := range paths.PathTo(id) {
			current = edge.Other(current)
			result.paths[id] = append(result.paths[id], current)
		}
	}
	return result, nil
}

// Distance returns shortest distance for a known node, +Inf when unreachable.
func (p *ShortestPaths[N]) Distance(node N) (float64, bool) {
	id, found := p.index.lookup(node)
	if !found {
		return 0, false
	}
	return p.distances[id], true
}

// Path returns a source-first node path. Unknown/unreachable nodes return false;
// the source path contains only source.
func (p *ShortestPaths[N]) Path(node N) ([]N, bool) {
	id, found := p.index.lookup(node)
	if !found || math.IsInf(p.distances[id], 1) {
		return nil, false
	}
	path := make([]N, 1, len(p.paths[id])+1)
	path[0] = p.nodes[p.source]
	for _, step := range p.paths[id] {
		path = append(path, p.nodes[step])
	}
	return path, true
}
