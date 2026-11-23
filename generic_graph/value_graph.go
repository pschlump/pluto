/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import "iter"

// ValueGraph associates a value with each unique endpoint pair. Edge values
// need not be unique.
type ValueGraph[N any, V any] struct {
	graph  *Graph[N]
	values map[pairKey]V
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
	return &ValueGraph[N, V]{graph: NewGraphFunc(config), values: make(map[pairKey]V)}
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
func (g *ValueGraph[N, V]) AddEdge(source, target N, value V) error {
	if err := g.graph.AddEdge(source, target); err != nil {
		return err
	}
	u, _ := g.graph.nodes.lookup(source)
	v, _ := g.graph.nodes.lookup(target)
	g.values[pairKey{u, v}] = value
	return nil
}

func (g *ValueGraph[N, V]) valueKey(source, target int) (pairKey, bool) {
	key := pairKey{source, target}
	if _, found := g.values[key]; found {
		return key, true
	}
	if !g.graph.config.Directed {
		key.source, key.target = key.target, key.source
		_, found := g.values[key]
		return key, found
	}
	return key, false
}

// SetEdgeValue changes an existing edge value without changing topology.
func (g *ValueGraph[N, V]) SetEdgeValue(source, target N, value V) error {
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		return err
	}
	key, found := g.valueKey(u, v)
	if !found {
		return ErrUnknownEdge
	}
	g.values[key] = value
	return nil
}

// EdgeValue returns the value associated with source-target.
func (g *ValueGraph[N, V]) EdgeValue(source, target N) (V, bool) {
	if g == nil || g.graph == nil {
		var zero V
		return zero, false
	}
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		var zero V
		return zero, false
	}
	key, found := g.valueKey(u, v)
	if !found {
		var zero V
		return zero, false
	}
	value := g.values[key]
	return value, true
}

func (g *ValueGraph[N, V]) RemoveEdge(source, target N) error {
	u, v, err := g.graph.ids(source, target)
	if err != nil {
		return err
	}
	if err := g.graph.RemoveEdge(source, target); err != nil {
		return err
	}
	key, _ := g.valueKey(u, v)
	delete(g.values, key)
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
	for key := range g.values {
		if key.source == id || key.target == id {
			delete(g.values, key)
		}
	}
	return g.graph.RemoveNode(node)
}

func (g *ValueGraph[N, V]) Copy() *ValueGraph[N, V] {
	if g == nil || g.graph == nil {
		return nil
	}
	result := &ValueGraph[N, V]{graph: g.graph.Copy(), values: make(map[pairKey]V, len(g.values))}
	for key, value := range g.values {
		result.values[key] = value
	}
	return result
}
