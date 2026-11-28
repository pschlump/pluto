/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package generic_graph

import (
	"iter"
	"slices"
)

type networkEdge struct {
	source, target int
	active         bool
}

// Network is a graph whose unique edge values are first-class identities.
// Parallel edges are supported when configured; antiparallel directed edges are
// distinct.
type Network[N any, E any] struct {
	config NetworkConfig[N, E]
	nodes  *indexedSet[N]
	edges  *indexedSet[E]
	links  []networkEdge
	// adj is parallel to nodes.entries: append-only incidence lists of edge
	// IDs (inactive IDs are skipped by readers); index/slots are unused.
	adj   []adjacency
	order []int
	count int
	arena idArena
}

func NewNetwork[N comparable, E comparable](directed, allowSelfLoops, allowParallelEdges bool) *Network[N, E] {
	return NewNetworkFunc(naturalNetworkConfig[N, E](directed, allowSelfLoops, allowParallelEdges))
}

func NewOrderedNetwork[N interface {
	comparable
	cmpOrdered
}, E interface {
	comparable
	cmpOrdered
}](directed, allowSelfLoops, allowParallelEdges bool) *Network[N, E] {
	config := naturalNetworkConfig[N, E](directed, allowSelfLoops, allowParallelEdges)
	config.Order = SortedOrder
	config.Less = func(a, b N) bool { return a < b }
	config.EdgeLess = func(a, b E) bool { return a < b }
	return NewNetworkFunc(config)
}

func NewNetworkFunc[N any, E any](config NetworkConfig[N, E]) *Network[N, E] {
	validateConfig(config.Config)
	if config.EqualEdge == nil || config.HashEdge == nil || config.ExpectedEdges < 0 || (config.Order == SortedOrder && config.EdgeLess == nil) {
		panic("generic_graph: invalid network edge functions/capacity")
	}
	return &Network[N, E]{config: config,
		nodes: newIndexedSet(config.Equal, config.Hash, config.ExpectedNodes, config.native),
		edges: newIndexedSet(config.EqualEdge, config.HashEdge, config.ExpectedEdges, config.nativeEdge),
		adj:   make([]adjacency, 0, config.ExpectedNodes), order: make([]int, 0, config.ExpectedEdges)}
}

func (n *Network[N, E]) IsDirected() bool          { return n != nil && n.config.Directed }
func (n *Network[N, E]) AllowsSelfLoops() bool     { return n != nil && n.config.AllowSelfLoops }
func (n *Network[N, E]) AllowsParallelEdges() bool { return n != nil && n.config.AllowParallelEdges }
func (n *Network[N, E]) AddNode(node N) error {
	if _, added := n.nodes.add(node); !added {
		return ErrDuplicateNode
	}
	n.adj = append(n.adj, adjacency{})
	return nil
}
func (n *Network[N, E]) ContainsNode(node N) bool { _, found := n.nodes.lookup(node); return found }
func (n *Network[N, E]) NodeCount() int {
	if n == nil || n.nodes == nil {
		return 0
	}
	return n.nodes.active
}
func (n *Network[N, E]) Len() int { return n.NodeCount() }
func (n *Network[N, E]) EdgeCount() int {
	if n == nil {
		return 0
	}
	return n.count
}
func (n *Network[N, E]) ContainsEdge(edge E) bool { _, found := n.edges.lookup(edge); return found }

func (n *Network[N, E]) AddEdge(source, target N, edge E) error {
	u, v, err := n.ids(source, target)
	if err != nil {
		return err
	}
	if u == v && !n.config.AllowSelfLoops {
		return ErrSelfLoop
	}
	if _, found := n.edges.lookup(edge); found {
		return ErrDuplicateEdge
	}
	if !n.config.AllowParallelEdges && n.hasEdgeIDs(u, v) {
		return ErrParallelEdge
	}
	id, _ := n.edges.add(edge)
	for len(n.links) <= id {
		n.links = append(n.links, networkEdge{})
	}
	n.links[id] = networkEdge{source: u, target: v, active: true}
	n.adj[u].out = n.arena.push(n.adj[u].out, id)
	n.adj[v].in = n.arena.push(n.adj[v].in, id)
	if !n.config.Directed && u != v {
		n.adj[v].out = n.arena.push(n.adj[v].out, id)
		n.adj[u].in = n.arena.push(n.adj[u].in, id)
	}
	n.order = append(n.order, id)
	n.count++
	return nil
}

func (n *Network[N, E]) ids(source, target N) (int, int, error) {
	if n == nil || n.nodes == nil {
		return 0, 0, ErrUnknownNode
	}
	u, found := n.nodes.lookup(source)
	if !found {
		return 0, 0, ErrUnknownNode
	}
	v, found := n.nodes.lookup(target)
	if !found {
		return 0, 0, ErrUnknownNode
	}
	return u, v, nil
}

func (n *Network[N, E]) hasEdgeIDs(source, target int) bool {
	for _, id := range n.adj[source].out {
		link := n.links[id]
		if link.active && (link.target == target || (!n.config.Directed && link.source == target)) {
			return true
		}
	}
	return false
}

func (n *Network[N, E]) HasEdge(source, target N) bool {
	u, v, err := n.ids(source, target)
	return err == nil && n.hasEdgeIDs(u, v)
}

// Endpoints returns the endpoint pair associated with edge.
func (n *Network[N, E]) Endpoints(edge E) (EndpointPair[N], bool) {
	id, found := n.edges.lookup(edge)
	if !found {
		return EndpointPair[N]{}, false
	}
	link := n.links[id]
	return EndpointPair[N]{n.nodes.entries[link.source].value, n.nodes.entries[link.target].value}, true
}

// CheckEndpoints verifies that edge has exactly source/target orientation.
func (n *Network[N, E]) CheckEndpoints(edge E, source, target N) error {
	u, v, err := n.ids(source, target)
	if err != nil {
		return err
	}
	id, found := n.edges.lookup(edge)
	if !found {
		return ErrUnknownEdge
	}
	link := n.links[id]
	if link.source != u || link.target != v {
		if n.config.Directed || link.source != v || link.target != u {
			return ErrInconsistentEndpoints
		}
	}
	return nil
}

func (n *Network[N, E]) RemoveEdge(edge E) error {
	id, found := n.edges.lookup(edge)
	if !found {
		return ErrUnknownEdge
	}
	n.links[id].active = false
	n.edges.remove(edge)
	n.count--
	return nil
}

func (n *Network[N, E]) RemoveNode(node N) error {
	id, found := n.nodes.lookup(node)
	if !found {
		return ErrUnknownNode
	}
	seen := make(map[int]struct{})
	for _, edgeID := range append(append([]int(nil), n.adj[id].out...), n.adj[id].in...) {
		if _, duplicate := seen[edgeID]; duplicate || !n.links[edgeID].active {
			continue
		}
		seen[edgeID] = struct{}{}
		n.links[edgeID].active = false
		n.edges.remove(n.edges.entries[edgeID].value)
		n.count--
	}
	n.adj[id] = adjacency{}
	n.nodes.remove(node)
	return nil
}

func (n *Network[N, E]) Nodes() iter.Seq[N] {
	if n == nil || n.nodes == nil {
		return sequence([]N(nil))
	}
	ids := make([]int, 0, n.nodes.active)
	for id := range n.nodes.entries {
		if n.nodes.hasID(id) {
			ids = append(ids, id)
		}
	}
	values := make([]N, 0, len(ids))
	for _, id := range ids {
		values = append(values, n.nodes.entries[id].value)
	}
	if n.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b N) int { return compareLess(a, b, n.config.Less) })
	}
	return sequence(values)
}
func (n *Network[N, E]) NodesSlice() []N { return slices.Collect(n.Nodes()) }

func (n *Network[N, E]) Edges() iter.Seq[E] {
	if n == nil || n.edges == nil {
		return sequence([]E(nil))
	}
	values := make([]E, 0, n.count)
	if n.config.Order == Unordered {
		for id, link := range n.links {
			if link.active {
				values = append(values, n.edges.entries[id].value)
			}
		}
	} else {
		for _, id := range n.order {
			if n.links[id].active {
				values = append(values, n.edges.entries[id].value)
			}
		}
	}
	if n.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b E) int { return compareLess(a, b, n.config.EdgeLess) })
	}
	return sequence(values)
}
func (n *Network[N, E]) EdgesSlice() []E { return slices.Collect(n.Edges()) }

func (n *Network[N, E]) IncidentEdges(node N) ([]E, error) {
	id, found := n.nodes.lookup(node)
	if !found {
		return nil, ErrUnknownNode
	}
	seen := make(map[int]struct{})
	values := make([]E, 0, len(n.adj[id].out)+len(n.adj[id].in))
	for _, edgeID := range append(append([]int(nil), n.adj[id].out...), n.adj[id].in...) {
		if _, duplicate := seen[edgeID]; duplicate || !n.links[edgeID].active {
			continue
		}
		seen[edgeID] = struct{}{}
		values = append(values, n.edges.entries[edgeID].value)
	}
	if n.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b E) int { return compareLess(a, b, n.config.EdgeLess) })
	}
	return values, nil
}

func (n *Network[N, E]) EdgesConnecting(source, target N) ([]E, error) {
	u, v, err := n.ids(source, target)
	if err != nil {
		return nil, err
	}
	values := make([]E, 0)
	for _, id := range n.adj[u].out {
		if n.links[id].active && n.links[id].target == v {
			values = append(values, n.edges.entries[id].value)
		}
	}
	if n.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b E) int { return compareLess(a, b, n.config.EdgeLess) })
	}
	return values, nil
}

func (n *Network[N, E]) neighbors(node N, predecessor bool) iter.Seq[N] {
	id, found := n.nodes.lookup(node)
	if !found {
		return sequence([]N(nil))
	}
	edgeIDs := n.adj[id].out
	if predecessor {
		edgeIDs = n.adj[id].in
	}
	seen := make(map[int]struct{})
	values := make([]N, 0, len(edgeIDs))
	for _, edgeID := range edgeIDs {
		link := n.links[edgeID]
		if !link.active {
			continue
		}
		other := link.target
		if predecessor {
			other = link.source
		}
		if _, duplicate := seen[other]; duplicate {
			continue
		}
		seen[other] = struct{}{}
		values = append(values, n.nodes.entries[other].value)
	}
	if n.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b N) int { return compareLess(a, b, n.config.Less) })
	}
	return sequence(values)
}
func (n *Network[N, E]) Successors(node N) iter.Seq[N]    { return n.neighbors(node, false) }
func (n *Network[N, E]) Predecessors(node N) iter.Seq[N]  { return n.neighbors(node, true) }
func (n *Network[N, E]) AdjacentNodes(node N) iter.Seq[N] { return n.Successors(node) }
func (n *Network[N, E]) SuccessorsSlice(node N) []N       { return slices.Collect(n.Successors(node)) }
func (n *Network[N, E]) PredecessorsSlice(node N) []N     { return slices.Collect(n.Predecessors(node)) }

func (n *Network[N, E]) Degree(node N) (int, error) {
	id, found := n.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	if n.config.Directed {
		return n.activeDegree(n.adj[id].out) + n.activeDegree(n.adj[id].in), nil
	}
	degree := n.activeDegree(n.adj[id].out)
	for _, edgeID := range n.adj[id].out {
		link := n.links[edgeID]
		if link.active && link.source == link.target {
			degree++
		}
	}
	return degree, nil
}

func (n *Network[N, E]) OutDegree(node N) (int, error) {
	id, found := n.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	return n.activeDegree(n.adj[id].out), nil
}
func (n *Network[N, E]) InDegree(node N) (int, error) {
	id, found := n.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	return n.activeDegree(n.adj[id].in), nil
}
func (n *Network[N, E]) activeDegree(ids []int) int {
	count := 0
	for _, id := range ids {
		if n.links[id].active {
			count++
		}
	}
	return count
}

func compareLess[T any](a, b T, less func(T, T) bool) int {
	if less(a, b) {
		return -1
	}
	if less(b, a) {
		return 1
	}
	return 0
}

func (n *Network[N, E]) Copy() *Network[N, E] {
	if n == nil || n.nodes == nil {
		return nil
	}
	return &Network[N, E]{config: n.config, nodes: n.nodes.copy(), edges: n.edges.copy(), links: append([]networkEdge(nil), n.links...),
		adj: copyAdjacency(n.adj), order: append([]int(nil), n.order...), count: n.count}
}
