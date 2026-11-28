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

// adjacency is one node's incidence lists, indexed by node ID in Graph.adj.
// out and in are exact (no stale IDs) and in edge-insertion order. An
// undirected graph keeps only out: its predecessor list would be the identical
// sequence, so inList returns out instead.
//
// Edge membership is answered from out alone: a linear scan while the list is
// short, and through index once it exceeds indexThreshold. index is an
// open-addressing table (power-of-two length, load <= 1/2) holding position+1
// into out, keyed by the neighbor ID stored at that position; 0 is empty.
// A valued graph keeps Graph.valueSlots[id] parallel to out, holding each
// edge's ValueGraph value slot (an undirected edge carries the same slot in
// both endpoints' lists); it lives outside adjacency so unvalued graphs and
// networks do not pay for the field.
type adjacency struct {
	out   []int
	in    []int
	index []int32
}

// Graph is a mutable directed or undirected graph with unique nodes and at most
// one edge per endpoint pair. Its zero value is an empty undirected graph that
// rejects writes until initialized by a constructor or Copy.
type Graph[N any] struct {
	config Config[N]
	nodes  *indexedSet[N]
	// adj is parallel to nodes.entries; removed nodes keep an empty entry.
	adj       []adjacency
	edgeOrder edgeLog // live edges only, insertion order and orientation
	edgeCount int
	// valued graphs (ValueGraph) maintain valueSlots (parallel to adj, each
	// list parallel to that node's out); slots of removed edges are recycled
	// through freeSlots.
	valued     bool
	valueSlots [][]int
	freeSlots  []int
	slots      int
	arena      idArena
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
	return &Graph[N]{config: config, nodes: newIndexedSet(config.Equal, config.Hash, config.ExpectedNodes, config.native),
		adj: make([]adjacency, 0, config.ExpectedNodes)}
}

// IsDirected reports the edge directionality.
func (g *Graph[N]) IsDirected() bool { return g != nil && g.config.Directed }

// AllowsSelfLoops reports the construction policy.
func (g *Graph[N]) AllowsSelfLoops() bool { return g != nil && g.config.AllowSelfLoops }

// AddNode adds an isolated node or returns ErrDuplicateNode.
// Complexity is O(1) expected (amortized).
func (g *Graph[N]) AddNode(node N) error {
	if g == nil || g.nodes == nil {
		return fmt.Errorf("%w: uninitialized graph", ErrUnknownNode)
	}
	if _, added := g.nodes.add(node); !added {
		return ErrDuplicateNode
	}
	g.adj = append(g.adj, adjacency{})
	if g.valued {
		g.valueSlots = append(g.valueSlots, nil)
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
	return g.edgeCount
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

// find returns the position of v in u's out list, or -1.
func (g *Graph[N]) find(u, v int) int {
	a := &g.adj[u]
	if a.index == nil {
		for i, id := range a.out {
			if id == v {
				return i
			}
		}
		return -1
	}
	mask := len(a.index) - 1
	for h := indexHash(v, mask); ; h = (h + 1) & mask {
		p := a.index[h]
		if p == 0 {
			return -1
		}
		if a.out[p-1] == v {
			return int(p - 1)
		}
	}
}

// indexThreshold is the out-list length above which a node gets an index.
const indexThreshold = 16

func indexHash(v, mask int) int {
	return int((uint64(v)*0x9E3779B97F4A7C15)>>32) & mask
}

// reindex rebuilds a's index from out, or drops it for a short list.
func reindex(a *adjacency) {
	if len(a.out) <= indexThreshold {
		a.index = nil
		return
	}
	size := len(a.index)
	if size < 2*len(a.out) || size > 8*len(a.out) {
		size = 4 * indexThreshold
		for size < 2*len(a.out) {
			size *= 2
		}
	}
	if size == len(a.index) {
		clear(a.index)
	} else {
		a.index = make([]int32, size)
	}
	mask := size - 1
	for i, v := range a.out {
		h := indexHash(v, mask)
		for a.index[h] != 0 {
			h = (h + 1) & mask
		}
		a.index[h] = int32(i + 1)
	}
}

// pushOut appends v (with slot when valued) to u's out list and indexes it.
func (g *Graph[N]) pushOut(u, v, slot int) {
	a := &g.adj[u]
	a.out = g.arena.push(a.out, v)
	if g.valued {
		g.valueSlots[u] = g.arena.push(g.valueSlots[u], slot)
	}
	if len(a.out) <= indexThreshold {
		return
	}
	if a.index == nil || 2*len(a.out) > len(a.index) {
		reindex(a)
		return
	}
	mask := len(a.index) - 1
	h := indexHash(v, mask)
	for a.index[h] != 0 {
		h = (h + 1) & mask
	}
	a.index[h] = int32(len(a.out))
}

// removeOutAt deletes position pos from u's out list (and slots) and returns
// the removed edge's slot.
func (g *Graph[N]) removeOutAt(u, pos int) int {
	a := &g.adj[u]
	a.out = slices.Delete(a.out, pos, pos+1)
	slot := 0
	if g.valued {
		slot = g.valueSlots[u][pos]
		g.valueSlots[u] = slices.Delete(g.valueSlots[u], pos, pos+1)
	}
	if a.index != nil {
		reindex(a)
	}
	return slot
}

// inList returns id's predecessor list; undirected graphs share out.
func (g *Graph[N]) inList(id int) []int {
	if g.config.Directed {
		return g.adj[id].in
	}
	return g.adj[id].out
}

// AddEdge adds source-target. Unknown nodes, duplicate edges, and disallowed
// self-loops return errors and do not mutate the graph.
// Complexity is O(1) expected (amortized).
func (g *Graph[N]) AddEdge(source, target N) error {
	u, v, err := g.ids(source, target)
	if err != nil {
		return err
	}
	_, err = g.addEdgeIDs(u, v)
	return err
}

// addEdgeIDs adds the edge u-v between live node IDs and returns its slot
// (always 0 for an unvalued graph).
func (g *Graph[N]) addEdgeIDs(u, v int) (int, error) {
	if u == v && !g.config.AllowSelfLoops {
		return 0, ErrSelfLoop
	}
	if g.find(u, v) >= 0 {
		return 0, ErrDuplicateEdge
	}
	slot := 0
	if g.valued {
		slot = g.slots
		if n := len(g.freeSlots); n > 0 {
			slot = g.freeSlots[n-1]
			g.freeSlots = g.freeSlots[:n-1]
		} else {
			g.slots++
		}
	}
	g.edgeOrder.push(pairKey{u, v})
	g.edgeCount++
	g.pushOut(u, v, slot)
	if g.config.Directed {
		g.adj[v].in = g.arena.push(g.adj[v].in, u)
	} else if u != v {
		g.pushOut(v, u, slot)
	}
	return slot, nil
}

// edgeSlot returns the value slot of edge u-v.
func (g *Graph[N]) edgeSlot(u, v int) (int, bool) {
	pos := g.find(u, v)
	if pos < 0 {
		return 0, false
	}
	return g.valueSlots[u][pos], true
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
// Complexity is O(E) for the insertion-order compaction plus O(degree).
func (g *Graph[N]) RemoveEdge(source, target N) error {
	u, v, err := g.ids(source, target)
	if err != nil {
		return err
	}
	_, err = g.removeEdgeIDs(u, v)
	return err
}

// removeEdgeIDs removes the edge between live IDs u and v, returning its slot.
func (g *Graph[N]) removeEdgeIDs(u, v int) (int, error) {
	pos := g.find(u, v)
	if pos < 0 {
		return 0, ErrUnknownEdge
	}
	slot := g.removeOutAt(u, pos)
	if g.config.Directed {
		g.adj[v].in = removeOrderedID(g.adj[v].in, u)
	} else if u != v {
		g.removeOutAt(v, g.find(v, u))
	}
	at := -1
	g.edgeOrder.all(func(i int, key pairKey) bool {
		if (key.source == u && key.target == v) || (!g.config.Directed && key.source == v && key.target == u) {
			at = i
			return false
		}
		return true
	})
	g.edgeOrder.deleteAt(at)
	g.edgeCount--
	g.releaseSlot(slot)
	return slot, nil
}

func (g *Graph[N]) releaseSlot(slot int) {
	if g.valued {
		g.freeSlots = append(g.freeSlots, slot)
	}
}

// HasEdge reports whether source-target exists. Unknown nodes return false.
// Complexity is O(1) expected.
func (g *Graph[N]) HasEdge(source, target N) bool {
	u, v, err := g.ids(source, target)
	if err != nil {
		return false
	}
	return g.find(u, v) >= 0
}

// RemoveNode removes node and every incident edge.
// Complexity is O(E) for the insertion-order compaction plus the degrees of
// the node's neighbors.
func (g *Graph[N]) RemoveNode(node N) error {
	if g == nil || g.nodes == nil {
		return ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return ErrUnknownNode
	}
	g.removeNodeID(id, nil)
	g.nodes.remove(node)
	return nil
}

// removeNodeID detaches every edge incident to id, reporting each freed slot
// to onSlot when it is non-nil. The caller removes id from the node index.
func (g *Graph[N]) removeNodeID(id int, onSlot func(int)) {
	free := func(slot int) {
		g.releaseSlot(slot)
		if onSlot != nil {
			onSlot(slot)
		}
	}
	a := g.adj[id]
	var slots []int
	if g.valued {
		slots = g.valueSlots[id]
		g.valueSlots[id] = nil
	}
	removed := len(a.out)
	for i, v := range a.out {
		if g.valued {
			free(slots[i])
		}
		if v == id {
			continue
		}
		if g.config.Directed {
			g.adj[v].in = removeOrderedID(g.adj[v].in, id)
		} else {
			g.removeOutAt(v, g.find(v, id))
		}
	}
	if g.config.Directed {
		for _, w := range a.in {
			if w == id {
				continue
			}
			removed++
			slot := g.removeOutAt(w, g.find(w, id))
			if g.valued {
				free(slot)
			}
		}
	}
	g.adj[id] = adjacency{}
	if removed > 0 {
		g.edgeCount -= removed
		g.edgeOrder.deleteFunc(func(key pairKey) bool {
			return key.source == id || key.target == id
		})
	}
}

func (g *Graph[N]) sortNodes(values []N) {
	if g.config.Order == SortedOrder {
		slices.SortFunc(values, func(a, b N) int { return compareLess(a, b, g.config.Less) })
	}
}

// Nodes returns a snapshot iterator in the configured order.
// Complexity is O(V) (plus O(V log V) when sorted).
func (g *Graph[N]) Nodes() iter.Seq[N] {
	return sequence(g.NodesSlice())
}

// NodesSlice materializes Nodes.
func (g *Graph[N]) NodesSlice() []N {
	if g == nil || g.nodes == nil || g.nodes.active == 0 {
		return nil
	}
	values := make([]N, 0, g.nodes.active)
	for _, entry := range g.nodes.entries {
		if entry.active {
			values = append(values, entry.value)
		}
	}
	g.sortNodes(values)
	return values
}

// neighborsSlice returns a fresh snapshot of node's successors or
// predecessors, or nil when there are none or node is unknown.
func (g *Graph[N]) neighborsSlice(node N, predecessor bool) []N {
	if g == nil || g.nodes == nil {
		return nil
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return nil
	}
	ids := g.adj[id].out
	if predecessor {
		ids = g.inList(id)
	}
	if len(ids) == 0 {
		return nil
	}
	values := make([]N, len(ids))
	for i, neighbor := range ids {
		values[i] = g.nodes.entries[neighbor].value
	}
	g.sortNodes(values)
	return values
}

// Successors returns outgoing (or undirected adjacent) nodes as a snapshot.
// Complexity is O(degree) (plus O(degree log degree) when sorted).
func (g *Graph[N]) Successors(node N) iter.Seq[N] { return sequence(g.neighborsSlice(node, false)) }

// Predecessors returns incoming nodes as a snapshot.
func (g *Graph[N]) Predecessors(node N) iter.Seq[N] { return sequence(g.neighborsSlice(node, true)) }

// AdjacentNodes is an alias for Successors.
func (g *Graph[N]) AdjacentNodes(node N) iter.Seq[N] { return g.Successors(node) }

// SuccessorsSlice materializes Successors.
func (g *Graph[N]) SuccessorsSlice(node N) []N { return g.neighborsSlice(node, false) }

// PredecessorsSlice materializes Predecessors.
func (g *Graph[N]) PredecessorsSlice(node N) []N { return g.neighborsSlice(node, true) }

// Degree returns incident edge count. Directed and undirected self-loops count
// twice, matching Guava. Complexity is O(1) expected.
func (g *Graph[N]) Degree(node N) (int, error) {
	if g == nil || g.nodes == nil {
		return 0, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	degree := len(g.adj[id].out)
	if g.config.Directed {
		degree += len(g.adj[id].in)
	} else if g.find(id, id) >= 0 {
		degree++
	}
	return degree, nil
}

// OutDegree returns the number of successors. Complexity is O(1) expected.
func (g *Graph[N]) OutDegree(node N) (int, error) {
	if g == nil || g.nodes == nil {
		return 0, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	return len(g.adj[id].out), nil
}

// InDegree returns the number of predecessors. Complexity is O(1) expected.
func (g *Graph[N]) InDegree(node N) (int, error) {
	if g == nil || g.nodes == nil {
		return 0, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return 0, ErrUnknownNode
	}
	return len(g.inList(id)), nil
}

// pairs snapshots the edges kept by keep (all when nil) as endpoint values.
func (g *Graph[N]) pairs(size int, keep func(pairKey) bool) []EndpointPair[N] {
	pairs := make([]EndpointPair[N], 0, size)
	g.edgeOrder.all(func(_ int, key pairKey) bool {
		if keep == nil || keep(key) {
			pairs = append(pairs, EndpointPair[N]{g.nodes.entries[key.source].value, g.nodes.entries[key.target].value})
		}
		return true
	})
	if g.config.Order == SortedOrder {
		slices.SortFunc(pairs, func(a, b EndpointPair[N]) int {
			if c := compareLess(a.Source, b.Source, g.config.Less); c != 0 {
				return c
			}
			return compareLess(a.Target, b.Target, g.config.Less)
		})
	}
	return pairs
}

// Edges returns a snapshot of unique endpoint pairs.
// Complexity is O(E) (plus O(E log E) when sorted).
func (g *Graph[N]) Edges() iter.Seq[EndpointPair[N]] {
	if g == nil || g.nodes == nil {
		return sequence([]EndpointPair[N](nil))
	}
	return sequence(g.pairs(g.edgeCount, nil))
}

// IncidentEdges returns unique edges entering or leaving node.
// Complexity is O(E).
func (g *Graph[N]) IncidentEdges(node N) ([]EndpointPair[N], error) {
	if g == nil || g.nodes == nil {
		return nil, ErrUnknownNode
	}
	id, found := g.nodes.lookup(node)
	if !found {
		return nil, ErrUnknownNode
	}
	degree := len(g.adj[id].out) + len(g.adj[id].in)
	return g.pairs(degree, func(key pairKey) bool { return key.source == id || key.target == id }), nil
}

// Copy returns an independent graph with the same configuration and order.
// Complexity is O(V + E); the adjacency lists share one allocation.
func (g *Graph[N]) Copy() *Graph[N] {
	if g == nil || g.nodes == nil {
		return nil
	}
	return &Graph[N]{config: g.config, nodes: g.nodes.copy(), adj: copyAdjacency(g.adj),
		edgeOrder: g.edgeOrder.clone(), edgeCount: g.edgeCount, valued: g.valued,
		valueSlots: copyLists(g.valueSlots), freeSlots: slices.Clone(g.freeSlots), slots: g.slots}
}

// copyAdjacency deep-copies adjacency lists into one backing array (plus one
// for the indexes). Each copy is capped at its length so a later append
// reallocates instead of writing into its neighbor's window.
func copyAdjacency(input []adjacency) []adjacency {
	total, indexed := 0, 0
	for _, a := range input {
		total += len(a.out) + len(a.in)
		indexed += len(a.index)
	}
	backing := make([]int, total)
	var indexBacking []int32
	if indexed > 0 {
		indexBacking = make([]int32, indexed)
	}
	output := make([]adjacency, len(input), cap(input))
	carve := func(ids []int) []int {
		if len(ids) == 0 {
			return nil
		}
		n := copy(backing, ids)
		window := backing[:n:n]
		backing = backing[n:]
		return window
	}
	for i, a := range input {
		output[i] = adjacency{out: carve(a.out), in: carve(a.in)}
		if n := len(a.index); n > 0 {
			output[i].index = indexBacking[:n:n]
			copy(output[i].index, a.index)
			indexBacking = indexBacking[n:]
		}
	}
	return output
}

// copyLists deep-copies lists into one backing array, capping each copy at
// its length (see copyAdjacency). A nil input stays nil.
func copyLists(input [][]int) [][]int {
	if input == nil {
		return nil
	}
	total := 0
	for _, list := range input {
		total += len(list)
	}
	backing := make([]int, total)
	output := make([][]int, len(input), cap(input))
	for i, list := range input {
		if len(list) > 0 {
			n := copy(backing, list)
			output[i] = backing[:n:n]
			backing = backing[n:]
		}
	}
	return output
}

// idArena hands out small adjacency-list windows from shared chunks, so a
// graph of many low-degree nodes costs a few chunk allocations instead of
// several growth allocations per list. Lists that outgrow arenaMaxWindow fall
// back to ordinary append growth. Windows never overlap: each is a
// three-index slice capped at its own end.
type idArena struct{ chunk []int }

const (
	arenaFirstWindow = 4
	arenaMaxWindow   = 16
	arenaChunk       = 2048
)

// push appends id to list, growing it from the arena when it is full.
func (a *idArena) push(list []int, id int) []int {
	if len(list) < cap(list) {
		return append(list, id)
	}
	n := max(arenaFirstWindow, 2*cap(list))
	if n > arenaMaxWindow {
		return append(list, id)
	}
	if len(a.chunk) < n {
		a.chunk = make([]int, arenaChunk)
	}
	grown := a.chunk[:len(list):n]
	a.chunk = a.chunk[n:]
	copy(grown, list)
	return append(grown, id)
}
