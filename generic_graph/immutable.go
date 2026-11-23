/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import "iter"

// ImmutableGraph is a defensive graph snapshot safe for concurrent reads.
type ImmutableGraph[N any] struct{ graph *Graph[N] }

func ImmutableGraphOf[N any](graph *Graph[N]) *ImmutableGraph[N] {
	if graph == nil {
		return nil
	}
	return &ImmutableGraph[N]{graph: graph.Copy()}
}
func (g *ImmutableGraph[N]) IsDirected() bool                 { return g != nil && g.graph.IsDirected() }
func (g *ImmutableGraph[N]) AllowsSelfLoops() bool            { return g != nil && g.graph.AllowsSelfLoops() }
func (g *ImmutableGraph[N]) ContainsNode(node N) bool         { return g.graph.ContainsNode(node) }
func (g *ImmutableGraph[N]) NodeCount() int                   { return g.graph.NodeCount() }
func (g *ImmutableGraph[N]) Len() int                         { return g.graph.NodeCount() }
func (g *ImmutableGraph[N]) EdgeCount() int                   { return g.graph.EdgeCount() }
func (g *ImmutableGraph[N]) Nodes() iter.Seq[N]               { return g.graph.Nodes() }
func (g *ImmutableGraph[N]) NodesSlice() []N                  { return g.graph.NodesSlice() }
func (g *ImmutableGraph[N]) Edges() iter.Seq[EndpointPair[N]] { return g.graph.Edges() }
func (g *ImmutableGraph[N]) Successors(node N) iter.Seq[N]    { return g.graph.Successors(node) }
func (g *ImmutableGraph[N]) Predecessors(node N) iter.Seq[N]  { return g.graph.Predecessors(node) }
func (g *ImmutableGraph[N]) AdjacentNodes(node N) iter.Seq[N] { return g.graph.AdjacentNodes(node) }
func (g *ImmutableGraph[N]) SuccessorsSlice(node N) []N       { return g.graph.SuccessorsSlice(node) }
func (g *ImmutableGraph[N]) PredecessorsSlice(node N) []N     { return g.graph.PredecessorsSlice(node) }
func (g *ImmutableGraph[N]) Degree(node N) (int, error)       { return g.graph.Degree(node) }
func (g *ImmutableGraph[N]) OutDegree(node N) (int, error)    { return g.graph.OutDegree(node) }
func (g *ImmutableGraph[N]) InDegree(node N) (int, error)     { return g.graph.InDegree(node) }
func (g *ImmutableGraph[N]) HasEdge(source, target N) bool    { return g.graph.HasEdge(source, target) }
func (g *ImmutableGraph[N]) IncidentEdges(node N) ([]EndpointPair[N], error) {
	return g.graph.IncidentEdges(node)
}
func (g *ImmutableGraph[N]) Traverser() *Traverser[N] { return ForGraph(g.graph) }
func (g *ImmutableGraph[N]) Transpose() *ImmutableGraph[N] {
	return ImmutableGraphOf(g.graph.Transpose())
}
func (g *ImmutableGraph[N]) TransitiveClosure(strategy ClosureSelfLoopStrategy) *ImmutableGraph[N] {
	return ImmutableGraphOf(g.graph.TransitiveClosure(strategy))
}
func (g *ImmutableGraph[N]) InducedSubgraph(nodes ...N) (*ImmutableGraph[N], error) {
	result, err := g.graph.InducedSubgraph(nodes...)
	if err != nil {
		return nil, err
	}
	return ImmutableGraphOf(result), nil
}

// ImmutableValueGraph is a defensive valued-graph snapshot.
type ImmutableValueGraph[N any, V any] struct{ graph *ValueGraph[N, V] }

func ImmutableValueGraphOf[N any, V any](graph *ValueGraph[N, V]) *ImmutableValueGraph[N, V] {
	if graph == nil {
		return nil
	}
	return &ImmutableValueGraph[N, V]{graph: graph.Copy()}
}
func (g *ImmutableValueGraph[N, V]) IsDirected() bool { return g != nil && g.graph.IsDirected() }
func (g *ImmutableValueGraph[N, V]) AllowsSelfLoops() bool {
	return g != nil && g.graph.AllowsSelfLoops()
}
func (g *ImmutableValueGraph[N, V]) ContainsNode(node N) bool         { return g.graph.ContainsNode(node) }
func (g *ImmutableValueGraph[N, V]) NodeCount() int                   { return g.graph.NodeCount() }
func (g *ImmutableValueGraph[N, V]) Len() int                         { return g.graph.NodeCount() }
func (g *ImmutableValueGraph[N, V]) EdgeCount() int                   { return g.graph.EdgeCount() }
func (g *ImmutableValueGraph[N, V]) Nodes() iter.Seq[N]               { return g.graph.Nodes() }
func (g *ImmutableValueGraph[N, V]) NodesSlice() []N                  { return g.graph.NodesSlice() }
func (g *ImmutableValueGraph[N, V]) Edges() iter.Seq[EndpointPair[N]] { return g.graph.Edges() }
func (g *ImmutableValueGraph[N, V]) Successors(node N) iter.Seq[N]    { return g.graph.Successors(node) }
func (g *ImmutableValueGraph[N, V]) Predecessors(node N) iter.Seq[N] {
	return g.graph.Predecessors(node)
}
func (g *ImmutableValueGraph[N, V]) AdjacentNodes(node N) iter.Seq[N] {
	return g.graph.AdjacentNodes(node)
}
func (g *ImmutableValueGraph[N, V]) SuccessorsSlice(node N) []N { return g.graph.SuccessorsSlice(node) }
func (g *ImmutableValueGraph[N, V]) PredecessorsSlice(node N) []N {
	return g.graph.PredecessorsSlice(node)
}
func (g *ImmutableValueGraph[N, V]) Degree(node N) (int, error)    { return g.graph.Degree(node) }
func (g *ImmutableValueGraph[N, V]) OutDegree(node N) (int, error) { return g.graph.OutDegree(node) }
func (g *ImmutableValueGraph[N, V]) InDegree(node N) (int, error)  { return g.graph.InDegree(node) }
func (g *ImmutableValueGraph[N, V]) HasEdge(source, target N) bool {
	return g.graph.HasEdge(source, target)
}
func (g *ImmutableValueGraph[N, V]) EdgeValue(source, target N) (V, bool) {
	return g.graph.EdgeValue(source, target)
}
func (g *ImmutableValueGraph[N, V]) IncidentEdges(node N) ([]EndpointPair[N], error) {
	return g.graph.IncidentEdges(node)
}
func (g *ImmutableValueGraph[N, V]) Traverser() *Traverser[N] { return ForValueGraph(g.graph) }
func (g *ImmutableValueGraph[N, V]) Transpose() *ImmutableValueGraph[N, V] {
	return ImmutableValueGraphOf(g.graph.Transpose())
}

// ImmutableNetwork is a defensive unique-edge network snapshot.
type ImmutableNetwork[N any, E any] struct{ network *Network[N, E] }

func ImmutableNetworkOf[N any, E any](network *Network[N, E]) *ImmutableNetwork[N, E] {
	if network == nil {
		return nil
	}
	return &ImmutableNetwork[N, E]{network: network.Copy()}
}
func (n *ImmutableNetwork[N, E]) IsDirected() bool { return n != nil && n.network.IsDirected() }
func (n *ImmutableNetwork[N, E]) AllowsSelfLoops() bool {
	return n != nil && n.network.AllowsSelfLoops()
}
func (n *ImmutableNetwork[N, E]) AllowsParallelEdges() bool {
	return n != nil && n.network.AllowsParallelEdges()
}
func (n *ImmutableNetwork[N, E]) ContainsNode(node N) bool      { return n.network.ContainsNode(node) }
func (n *ImmutableNetwork[N, E]) ContainsEdge(edge E) bool      { return n.network.ContainsEdge(edge) }
func (n *ImmutableNetwork[N, E]) NodeCount() int                { return n.network.NodeCount() }
func (n *ImmutableNetwork[N, E]) Len() int                      { return n.network.NodeCount() }
func (n *ImmutableNetwork[N, E]) EdgeCount() int                { return n.network.EdgeCount() }
func (n *ImmutableNetwork[N, E]) Nodes() iter.Seq[N]            { return n.network.Nodes() }
func (n *ImmutableNetwork[N, E]) NodesSlice() []N               { return n.network.NodesSlice() }
func (n *ImmutableNetwork[N, E]) Edges() iter.Seq[E]            { return n.network.Edges() }
func (n *ImmutableNetwork[N, E]) EdgesSlice() []E               { return n.network.EdgesSlice() }
func (n *ImmutableNetwork[N, E]) Successors(node N) iter.Seq[N] { return n.network.Successors(node) }
func (n *ImmutableNetwork[N, E]) Predecessors(node N) iter.Seq[N] {
	return n.network.Predecessors(node)
}
func (n *ImmutableNetwork[N, E]) AdjacentNodes(node N) iter.Seq[N] {
	return n.network.AdjacentNodes(node)
}
func (n *ImmutableNetwork[N, E]) SuccessorsSlice(node N) []N { return n.network.SuccessorsSlice(node) }
func (n *ImmutableNetwork[N, E]) PredecessorsSlice(node N) []N {
	return n.network.PredecessorsSlice(node)
}
func (n *ImmutableNetwork[N, E]) Degree(node N) (int, error)    { return n.network.Degree(node) }
func (n *ImmutableNetwork[N, E]) OutDegree(node N) (int, error) { return n.network.OutDegree(node) }
func (n *ImmutableNetwork[N, E]) InDegree(node N) (int, error)  { return n.network.InDegree(node) }
func (n *ImmutableNetwork[N, E]) HasEdge(source, target N) bool {
	return n.network.HasEdge(source, target)
}
func (n *ImmutableNetwork[N, E]) Endpoints(edge E) (EndpointPair[N], bool) {
	return n.network.Endpoints(edge)
}
func (n *ImmutableNetwork[N, E]) CheckEndpoints(edge E, source, target N) error {
	return n.network.CheckEndpoints(edge, source, target)
}
func (n *ImmutableNetwork[N, E]) IncidentEdges(node N) ([]E, error) {
	return n.network.IncidentEdges(node)
}
func (n *ImmutableNetwork[N, E]) EdgesConnecting(source, target N) ([]E, error) {
	return n.network.EdgesConnecting(source, target)
}
func (n *ImmutableNetwork[N, E]) Traverser() *Traverser[N] { return ForNetwork(n.network) }
func (n *ImmutableNetwork[N, E]) Transpose() *ImmutableNetwork[N, E] {
	return ImmutableNetworkOf(n.network.Transpose())
}
