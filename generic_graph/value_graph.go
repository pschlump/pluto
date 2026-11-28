/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import (
	"iter"
	"slices"
)

// ValueGraph associates a value with each unique endpoint pair. Edge values
// need not be unique.
type ValueGraph[N any, V any] struct {
	graph *Graph[N]
	// values is indexed by the edge's slot (graph.adj[*].slots); freed slots
	// are zeroed so they retain nothing.
	values []V
}

func NewValueGraph[N comparable, V any](directed, allowSelfLoops bool) *ValueGraph[N, V] {
	return NewValueGraphFunc[N, V](naturalConfig[N](directed, allowSelfLoops))
}

func NewOrderedValueGraph[N interface {
	comparable
	cmpOrdered
}, V any](directed, allowSelfLoops bool) *ValueGraph[N, V] {
	config := naturalConfig[N](directed, allowSelfLoops)
	config.Order = SortedOrder
	config.Less = func(a, b N) bool { return a < b }
	return NewValueGraphFunc[N, V](config)
}

func NewValueGraphFunc[N any, V any](config Config[N]) *ValueGraph[N, V] {
	graph := NewGraphFunc(config)
	graph.valued = true
	return &ValueGraph[N, V]{graph: graph}
}

func (g *ValueGraph[N, V]) IsDirected() bool                 { return g != nil && g.graph.IsDirected() }
func (g *ValueGraph[N, V]) AllowsSelfLoops() bool            { return g != nil && g.graph.AllowsSelfLoops() }
func (g *ValueGraph[N, V]) AddNode(node N) error             { return g.graph.AddNode(node) }
func (g *ValueGraph[N, V]) ContainsNode(node N) bool         { return g.graph.ContainsNode(node) }
func (g *ValueGraph[N, V]) NodeCount() int                   { return g.graph.NodeCount() }
func (g *ValueGraph[N, V]) Len() int                         { return g.graph.NodeCount() }
func (g *ValueGraph[N, V]) EdgeCount() int                   { return g.graph.EdgeCount() }
func (g *ValueGraph[N, V]) Nodes() iter.Seq[N]               { return g.graph.Nodes() }
func (g *ValueGraph[N, V]) NodesSlice() []N                  { return g.graph.NodesSlice() }
func (g *ValueGraph[N, V]) Edges() iter.Seq[EndpointPair[N]] { return g.graph.Edges() }
func (g *ValueGraph[N, V]) Successors(node N) iter.Seq[N]    { return g.graph.Successors(node) }
func (g *ValueGraph[N, V]) Predecessors(node N) iter.Seq[N]  { return g.graph.Predecessors(node) }
func (g *ValueGraph[N, V]) AdjacentNodes(node N) iter.Seq[N] { return g.graph.AdjacentNodes(node) }
func (g *ValueGraph[N, V]) SuccessorsSlice(node N) []N       { return g.graph.SuccessorsSlice(node) }
func (g *ValueGraph[N, V]) PredecessorsSlice(node N) []N     { return g.graph.PredecessorsSlice(node) }
func (g *ValueGraph[N, V]) Degree(node N) (int, error)       { return g.graph.Degree(node) }
func (g *ValueGraph[N, V]) OutDegree(node N) (int, error)    { return g.graph.OutDegree(node) }
func (g *ValueGraph[N, V]) InDegree(node N) (int, error)     { return g.graph.InDegree(node) }
func (g *ValueGraph[N, V]) HasEdge(source, target N) bool    { return g.graph.HasEdge(source, target) }
func (g *ValueGraph[N, V]) IncidentEdges(node N) ([]EndpointPair[N], error) {
	return g.graph.IncidentEdges(node)
}

// AddEdge adds source-target with value and rejects duplicate edges.
// Complexity is O(1) expected (amortized).
func (g *ValueGraph[N, V]) AddEdge(source, target N, value V) error {
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		return err
	}
	slot, err := g.graph.addEdgeIDs(u, v)
	if err != nil {
		return err
	}
	g.setSlot(slot, value)
	return nil
}

// setSlot stores value in slot, growing values when slot is new.
func (g *ValueGraph[N, V]) setSlot(slot int, value V) {
	if slot >= len(g.values) {
		g.values = slices.Grow(g.values, slot+1-len(g.values))[:slot+1]
	}
	g.values[slot] = value
}

// SetEdgeValue changes an existing edge value without changing topology.
func (g *ValueGraph[N, V]) SetEdgeValue(source, target N, value V) error {
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		return err
	}
	slot, found := g.graph.edgeSlot(u, v)
	if !found {
		return ErrUnknownEdge
	}
	g.values[slot] = value
	return nil
}

// EdgeValue returns the value associated with source-target.
// Complexity is O(1) expected.
func (g *ValueGraph[N, V]) EdgeValue(source, target N) (V, bool) {
	var zero V
	if g == nil || g.graph == nil {
		return zero, false
	}
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		return zero, false
	}
	slot, found := g.graph.edgeSlot(u, v)
	if !found {
		return zero, false
	}
	return g.values[slot], true
}

func (g *ValueGraph[N, V]) RemoveEdge(source, target N) error {
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		return err
	}
	slot, err := g.graph.removeEdgeIDs(u, v)
	if err != nil {
		return err
	}
	var zero V
	g.values[slot] = zero
	return nil
}

func (g *ValueGraph[N, V]) RemoveNode(node N) error {
	if g == nil || g.graph == nil || g.graph.nodes == nil {
		return ErrUnknownNode
	}
	id, found := g.graph.nodes.lookup(node)
	if !found {
		return ErrUnknownNode
	}
	g.graph.removeNodeID(id, func(slot int) {
		var zero V
		g.values[slot] = zero
	})
	g.graph.nodes.remove(node)
	return nil
}

func (g *ValueGraph[N, V]) Copy() *ValueGraph[N, V] {
	if g == nil || g.graph == nil {
		return nil
	}
	return &ValueGraph[N, V]{graph: g.graph.Copy(), values: slices.Clone(g.values)}
}
