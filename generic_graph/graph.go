/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import (
	"fmt"
	"iter"
	"slices"
)

type pairKey struct{ source, target int }

// Graph is a mutable directed or undirected graph with unique nodes and at most
// one edge per endpoint pair. Its zero value is an empty undirected graph that
// rejects writes until initialized by a constructor or Copy.
type Graph[N any] struct {
	config    Config[N]
	nodes     *indexedSet[N]
	out       map[int]map[int]struct{}
	in        map[int]map[int]struct{}
	outOrder  map[int][]int
	inOrder   map[int][]int
	edges     map[pairKey]struct{}
	edgeOrder []pairKey
}

// NewGraph constructs a naturally indexed graph with insertion-order iteration.
func NewGraph[N comparable](directed, allowSelfLoops bool) *Graph[N] {
	return NewGraphFunc(naturalConfig[N](directed, allowSelfLoops))
}

// NewOrderedGraph constructs a naturally indexed graph with sorted iteration.
func NewOrderedGraph[N interface {
	comparable
	cmpOrdered
}](directed, allowSelfLoops bool) *Graph[N] {
	config := naturalConfig[N](directed, allowSelfLoops)
	config.Order = SortedOrder
	config.Less = func(a, b N) bool { return a < b }
	return NewGraphFunc(config)
}

// cmpOrdered is the local form of cmp.Ordered used by NewOrderedGraph.
type cmpOrdered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string
}

// NewGraphFunc constructs a graph with explicit equality/hash/order functions.
func NewGraphFunc[N any](config Config[N]) *Graph[N] {
	validateConfig(config)
	return &Graph[N]{config: config, nodes: newIndexedSet(config.Equal, config.Hash, config.ExpectedNodes),
		out: make(map[int]map[int]struct{}), in: make(map[int]map[int]struct{}),
		outOrder: make(map[int][]int), inOrder: make(map[int][]int), edges: make(map[pairKey]struct{})}
}

// IsDirected reports the edge directionality.
func (g *Graph[N]) IsDirected() bool { return g != nil && g.config.Directed }

// AllowsSelfLoops reports the construction policy.
func (g *Graph[N]) AllowsSelfLoops() bool { return g != nil && g.config.AllowSelfLoops }

// AddNode adds an isolated node or returns ErrDuplicateNode.
func (g *Graph[N]) AddNode(node N) error {
	if g == nil || g.nodes == nil {
		return fmt.Errorf("%w: uninitialized graph", ErrUnknownNode)
	}
	if _, added := g.nodes.add(node); !added {
		return ErrDuplicateNode
	}
	return nil
}

// ContainsNode reports whether node exists.
func (g *Graph[N]) ContainsNode(node N) bool {
	if g == nil {
		return false
	}
	_, found := g.nodes.lookup(node)
	return found
}

// NodeCount returns the number of nodes, including isolated nodes.
func (g *Graph[N]) NodeCount() int {
	if g == nil || g.nodes == nil {
		return 0
	}
	return g.nodes.active
}

// Len is an alias for NodeCount.
func (g *Graph[N]) Len() int { return g.NodeCount() }

// EdgeCount returns unique endpoint-pair edges; a self-loop counts once.
func (g *Graph[N]) EdgeCount() int {
	if g == nil {
		return 0
	}
	return len(g.edges)
}

func (g *Graph[N]) ids(source, target N) (int, int, error) {
	if g == nil || g.nodes == nil {
		return 0, 0, ErrUnknownNode
	}
	u, found := g.nodes.lookup(source)
	if !found {
		return 0, 0, fmt.Errorf("%w: source", ErrUnknownNode)
	}
	v, found := g.nodes.lookup(target)
	if !found {
		return 0, 0, fmt.Errorf("%w: target", ErrUnknownNode)
	}
	return u, v, nil
}

// AddEdge adds source-target. Unknown nodes, duplicate edges, and disallowed
// self-loops return errors and do not mutate the graph.
func (g *Graph[N]) AddEdge(source, target N) error {
	u, v, err := g.ids(source, target)
	if err != nil {
		return err
	}
	if u == v && !g.config.AllowSelfLoops {
		return ErrSelfLoop
	}
	key := pairKey{u, v}
	if _, found := g.out[u][v]; found {
		return ErrDuplicateEdge
	}
	g.edges[key] = struct{}{}
	g.edgeOrder = append(g.edgeOrder, key)
	g.connect(u, v)
	if !g.config.Directed && u != v {
		g.connect(v, u)
	}
	return nil
}

func (g *Graph[N]) connect(source, target int) {
	if g.out[source] == nil {
		g.out[source] = make(map[int]struct{})
	}
	if g.in[target] == nil {
		g.in[target] = make(map[int]struct{})
	}
	g.out[source][target] = struct{}{}
	g.in[target][source] = struct{}{}
	g.outOrder[source] = append(g.outOrder[source], target)
	g.inOrder[target] = append(g.inOrder[target], source)
}

func (g *Graph[N]) disconnect(source, target int) {
	delete(g.out[source], target)
	delete(g.in[target], source)
	g.outOrder[source] = removeOrderedID(g.outOrder[source], target)
	g.inOrder[target] = removeOrderedID(g.inOrder[target], source)
}

func removeOrderedID(ids []int, target int) []int {
	for i, id := range ids {
		if id == target {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}

// RemoveEdge removes source-target or returns an unknown-node/edge error.
func (g *Graph[N]) RemoveEdge(source, target N) error {
	u, v, err := g.ids(source, target)
	if err != nil {
		return err
	}
	key := pairKey{u, v}
	if _, found := g.edges[key]; !found && !g.config.Directed {
		key.source, key.target = key.target, key.source
	}
	if _, found := g.edges[key]; !found {
		return ErrUnknownEdge
	}
	delete(g.edges, key)
	for i, existing := range g.edgeOrder {
		if existing == key {
			g.edgeOrder = append(g.edgeOrder[:i], g.edgeOrder[i+1:]...)
			break
		}
	}
	g.disconnect(key.source, key.target)
	if !g.config.Directed && key.source != key.target {
		g.disconnect(key.target, key.source)
	}
	return nil
}

// HasEdge reports whether source-target exists. Unknown nodes return false.
func (g *Graph[N]) HasEdge(source, target N) bool {
	u, v, err := g.ids(source, target)
	if err != nil {
		return false
	}
	_, found := g.out[u][v]
	return found
}

// RemoveNode removes node and every incident edge.
func (g *Graph[N]) RemoveNode(node N) error {
	if g == nil || g.nodes == nil {
		return ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return ErrUnknownNode
	}
	for key := range g.edges {
		if key.source == id || key.target == id {
			delete(g.edges, key)
		}
	}
	for neighbor := range g.out[id] {
		delete(g.in[neighbor], id)
	}
	for neighbor := range g.in[id] {
		delete(g.out[neighbor], id)
	}
	delete(g.out, id)
	delete(g.in, id)
	delete(g.outOrder, id)
	delete(g.inOrder, id)
	g.nodes.remove(node)
	return nil
}

func (g *Graph[N]) nodeValues(ids []int) []N {
	values := make([]N, 0, len(ids))
	for _, id := range ids {
		values = append(values, g.nodes.entries[id].value)
	}
	if g.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b N) int {
			if g.config.Less(a, b) {
				return -1
			}
			if g.config.Less(b, a) {
				return 1
			}
			return 0
		})
	}
	return values
}

func (g *Graph[N]) activeIDs() []int {
	ids := make([]int, 0, g.NodeCount())
	for id := range g.nodes.entries {
		if g.nodes.hasID(id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// Nodes returns a snapshot iterator in the configured order.
func (g *Graph[N]) Nodes() iter.Seq[N] {
	if g == nil || g.nodes == nil {
		return sequence([]N(nil))
	}
	return sequence(g.nodeValues(g.activeIDs()))
}

// NodesSlice materializes Nodes.
func (g *Graph[N]) NodesSlice() []N {
	return slices.Collect(g.Nodes())
}

func (g *Graph[N]) neighborIDs(id int, predecessor bool) []int {
	table := g.out[id]
	order := g.outOrder[id]
	if predecessor {
		table = g.in[id]
		order = g.inOrder[id]
	}
	ids := make([]int, 0, len(table))
	if g.config.Order == Unordered {
		for neighbor := range table {
			ids = append(ids, neighbor)
		}
		return ids
	}
	for _, neighbor := range order {
		if _, found := table[neighbor]; found {
			ids = append(ids, neighbor)
		}
	}
	return ids
}

func (g *Graph[N]) neighbors(node N, predecessor bool) iter.Seq[N] {
	if g == nil || g.nodes == nil {
		return sequence([]N(nil))
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return sequence([]N(nil))
	}
	return sequence(g.nodeValues(g.neighborIDs(id, predecessor)))
}

// Successors returns outgoing (or undirected adjacent) nodes as a snapshot.
func (g *Graph[N]) Successors(node N) iter.Seq[N] { return g.neighbors(node, false) }

// Predecessors returns incoming nodes as a snapshot.
func (g *Graph[N]) Predecessors(node N) iter.Seq[N] { return g.neighbors(node, true) }

// AdjacentNodes is an alias for Successors.
func (g *Graph[N]) AdjacentNodes(node N) iter.Seq[N] { return g.Successors(node) }

// SuccessorsSlice materializes Successors.
func (g *Graph[N]) SuccessorsSlice(node N) []N { return slices.Collect(g.Successors(node)) }

// PredecessorsSlice materializes Predecessors.
func (g *Graph[N]) PredecessorsSlice(node N) []N { return slices.Collect(g.Predecessors(node)) }

// Degree returns incident edge count. Directed and undirected self-loops count
// twice, matching Guava.
func (g *Graph[N]) Degree(node N) (int, error) {
	if g == nil || g.nodes == nil {
		return 0, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	degree := len(g.out[id])
	if g.config.Directed {
		degree += len(g.in[id])
	} else if _, loop := g.out[id][id]; loop {
		degree++
	}
	return degree, nil
}

func (g *Graph[N]) OutDegree(node N) (int, error) {
	if g == nil || g.nodes == nil {
		return 0, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	return len(g.out[id]), nil
}

func (g *Graph[N]) InDegree(node N) (int, error) {
	if g == nil || g.nodes == nil {
		return 0, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	return len(g.in[id]), nil
}

// Edges returns a snapshot of unique endpoint pairs.
func (g *Graph[N]) Edges() iter.Seq[EndpointPair[N]] {
	if g == nil {
		return sequence([]EndpointPair[N](nil))
	}
	keys := make([]pairKey, 0, len(g.edges))
	if g.config.Order == Unordered {
		for key := range g.edges {
			keys = append(keys, key)
		}
	} else {
		for _, key := range g.edgeOrder {
			if _, found := g.edges[key]; found {
				keys = append(keys, key)
			}
		}
	}
	pairs := make([]EndpointPair[N], 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, EndpointPair[N]{g.nodes.entries[key.source].value, g.nodes.entries[key.target].value})
	}
	if g.config.Order == SortedOrder {
		slices.SortFunc(pairs, func(a, b EndpointPair[N]) int {
			if g.config.Less(a.Source, b.Source) {
				return -1
			}
			if g.config.Less(b.Source, a.Source) {
				return 1
			}
			if g.config.Less(a.Target, b.Target) {
				return -1
			}
			if g.config.Less(b.Target, a.Target) {
				return 1
			}
			return 0
		})
	}
	return sequence(pairs)
}

// IncidentEdges returns unique edges entering or leaving node.
func (g *Graph[N]) IncidentEdges(node N) ([]EndpointPair[N], error) {
	if g == nil || g.nodes == nil {
		return nil, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return nil, ErrUnknownNode
	}
	pairs := make([]EndpointPair[N], 0)
	for pair := range g.Edges() {
		pairID := pairKey{}
		pairID.source, _ = g.nodes.lookup(pair.Source)
		pairID.target, _ = g.nodes.lookup(pair.Target)
		if pairID.source == id || pairID.target == id {
			pairs = append(pairs, pair)
		}
	}
	return pairs, nil
}

// Copy returns an independent graph with the same configuration and order.
func (g *Graph[N]) Copy() *Graph[N] {
	if g == nil || g.nodes == nil {
		return nil
	}
	copyGraph := &Graph[N]{config: g.config, nodes: g.nodes.copy(), edges: make(map[pairKey]struct{}, len(g.edges)),
		edgeOrder: append([]pairKey(nil), g.edgeOrder...), out: copyAdjacency(g.out), in: copyAdjacency(g.in),
		outOrder: copyOrder(g.outOrder), inOrder: copyOrder(g.inOrder)}
	for key := range g.edges {
		copyGraph.edges[key] = struct{}{}
	}
	return copyGraph
}

func copyAdjacency(input map[int]map[int]struct{}) map[int]map[int]struct{} {
	output := make(map[int]map[int]struct{}, len(input))
	for id, neighbors := range input {
		output[id] = make(map[int]struct{}, len(neighbors))
		for neighbor := range neighbors {
			output[id][neighbor] = struct{}{}
		}
	}
	return output
}

func copyOrder(input map[int][]int) map[int][]int {
	output := make(map[int][]int, len(input))
	for id, order := range input {
		output[id] = append([]int(nil), order...)
	}
	return output
}
