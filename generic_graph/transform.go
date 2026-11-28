/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import "slices"

// ClosureSelfLoopStrategy controls self-loops added by TransitiveClosure.
type ClosureSelfLoopStrategy uint8

const (
	// AddSelfLoopsAlways treats every zero-length path as a cycle.
	AddSelfLoopsAlways ClosureSelfLoopStrategy = iota
	// AddSelfLoopsForCycles adds a self-loop only for a nonempty cycle. Existing
	// self-loops are preserved under either strategy.
	AddSelfLoopsForCycles
)

// Transpose returns an independent snapshot with every directed edge reversed.
func (g *Graph[N]) Transpose() *Graph[N] {
	if g == nil {
		return nil
	}
	if !g.config.Directed {
		return g.Copy()
	}
	result := NewGraphFunc(g.config)
	for _, node := range g.NodesSlice() {
		_ = result.AddNode(node)
	}
	for pair := range g.Edges() {
		_ = result.AddEdge(pair.Target, pair.Source)
	}
	return result
}

func (g *ValueGraph[N, V]) Transpose() *ValueGraph[N, V] {
	if g == nil {
		return nil
	}
	if !g.graph.config.Directed {
		return g.Copy()
	}
	result := NewValueGraphFunc[N, V](g.graph.config)
	for _, node := range g.NodesSlice() {
		_ = result.AddNode(node)
	}
	for pair := range g.Edges() {
		value, _ := g.EdgeValue(pair.Source, pair.Target)
		_ = result.AddEdge(pair.Target, pair.Source, value)
	}
	return result
}

func (n *Network[N, E]) Transpose() *Network[N, E] {
	if n == nil {
		return nil
	}
	if !n.config.Directed {
		return n.Copy()
	}
	result := NewNetworkFunc(n.config)
	for _, node := range n.NodesSlice() {
		_ = result.AddNode(node)
	}
	for _, edge := range n.EdgesSlice() {
		pair, _ := n.Endpoints(edge)
		_ = result.AddEdge(pair.Target, pair.Source, edge)
	}
	return result
}

// InducedSubgraph copies selected nodes and edges whose endpoints are both
// selected. Unknown or duplicate input nodes return an error.
func (g *Graph[N]) InducedSubgraph(nodes ...N) (*Graph[N], error) {
	if g == nil {
		return nil, nil
	}
	result := NewGraphFunc(g.config)
	for _, node := range nodes {
		if !g.ContainsNode(node) {
			return nil, ErrUnknownNode
		}
		if err := result.AddNode(node); err != nil {
			return nil, err
		}
	}
	for pair := range g.Edges() {
		if result.ContainsNode(pair.Source) && result.ContainsNode(pair.Target) {
			_ = result.AddEdge(pair.Source, pair.Target)
		}
	}
	return result, nil
}

func (g *ValueGraph[N, V]) InducedSubgraph(nodes ...N) (*ValueGraph[N, V], error) {
	if g.graph == nil {
		return nil, nil
	}
	result := NewValueGraphFunc[N, V](g.graph.config)
	for _, node := range nodes {
		if !g.ContainsNode(node) {
			return nil, ErrUnknownNode
		}
		if err := result.AddNode(node); err != nil {
			return nil, err
		}
	}
	for pair := range g.Edges() {
		if result.ContainsNode(pair.Source) && result.ContainsNode(pair.Target) {
			value, _ := g.EdgeValue(pair.Source, pair.Target)
			_ = result.AddEdge(pair.Source, pair.Target, value)
		}
	}
	return result, nil
}

func (n *Network[N, E]) InducedSubgraph(nodes ...N) (*Network[N, E], error) {
	if n == nil {
		return nil, nil
	}
	result := NewNetworkFunc(n.config)
	for _, node := range nodes {
		if !n.ContainsNode(node) {
			return nil, ErrUnknownNode
		}
		if err := result.AddNode(node); err != nil {
			return nil, err
		}
	}
	for _, edge := range n.EdgesSlice() {
		pair, _ := n.Endpoints(edge)
		if result.ContainsNode(pair.Source) && result.ContainsNode(pair.Target) {
			_ = result.AddEdge(pair.Source, pair.Target, edge)
		}
	}
	return result, nil
}

// TransitiveClosure returns a graph snapshot containing every reachable pair.
func (g *Graph[N]) TransitiveClosure(strategy ClosureSelfLoopStrategy) *Graph[N] {
	if g == nil {
		return nil
	}
	if strategy > AddSelfLoopsForCycles {
		panic("generic_graph: unknown closure self-loop strategy")
	}
	config := g.config
	config.AllowSelfLoops = true
	result := NewGraphFunc(config)
	nodes := g.NodesSlice()
	for _, node := range nodes {
		_ = result.AddNode(node)
	}
	for _, node := range nodes {
		starts := []N{node}
		if strategy == AddSelfLoopsForCycles {
			starts = g.SuccessorsSlice(node)
		}
		reachable := slices.Collect(ForGraph(g).BreadthFirst(starts...))
		for _, target := range reachable {
			if !result.HasEdge(node, target) {
				_ = result.AddEdge(node, target)
			}
		}
	}
	return result
}
