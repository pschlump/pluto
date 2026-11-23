# generic_graph

## Data model and complexity

`Graph[N]` stores unique nodes in a stable-ID `hash_grow` index. Internal IDs
are never reused, so removal cannot make stale adjacency history live again.
Outgoing/incoming neighbor sets provide expected O(1) edge lookup; insertion-order
lists are compacted on edge removal. Node and edge add/remove are expected O(1)
apart from edge-history compaction; node removal is O(incident edges). Nodes and
edges retain isolated values and insertion order. SortedOrder sorts snapshot
results with Config.Less; Unordered makes no order guarantee.

Graph permits one edge per unordered endpoint pair when undirected and per
ordered pair when directed. Reverse undirected Add/Has/Remove address the same
edge. A self-loop counts once in EdgeCount and twice in Degree; directed InDegree
and OutDegree each count it once. Mutation returns sentinel errors for unknown /
duplicate nodes/edges, loops, and inconsistent operations.

`ValueGraph[N,V]` adds a pair-keyed value map and keeps it synchronized through
node/edge removal. `Network[N,E]` has a second stable-ID index for unique edge
objects, supports configurable parallel edges and antiparallel directed edges,
and exposes Endpoints, CheckEndpoints, IncidentEdges, and EdgesConnecting.
Network append-only incidence lists skip inactive IDs and count degrees from
active links only.

## Traversal, transformations, and immutability

`ForGraph`, `ForValueGraph`, and `ForNetwork` deduplicate through the same custom
equality/hash functions. BFS/DFS pre/postorder are lazy and stop callers'
successor iteration early. `ForFunc` is tree-only: no visited set, so cycles or
shared children violate its precondition. Recursive DFS can consume stack in a
very deep graph.

ReachableNodes includes its source. TransitiveClosure supports
AddSelfLoopsAlways and AddSelfLoopsForCycles, preserving existing loops exactly
as Guava. Transpose, InducedSubgraph, Copy, and closure are independent snapshots,
not live views. ImmutableGraphOf/ImmutableValueGraphOf/ImmutableNetworkOf make
defensive copies with no mutation surface; concurrent reads are race-tested.

No mutable `_ts` wrappers are provided. Use immutable snapshots for concurrent
sharing; the existing graph_ts/digraph_ts packages remain unchanged.

## Adapters and verification

Dense assigns IDs in configured node order and builds independent graph/digraph
snapshots; empty graphs return nil classic pointers because their constructors
require vertices. Dijkstra accepts ValueGraph[N,float64], rejects negative/NaN
weights, and maps distances and source-first node paths back to N. Adapter work
is benchmarked separately from the integer algorithm.

Fixed-seed randomized graph mutations compare against adjacency maps. Tests cover
loop degree, reverse undirected operations, edge reinsertion order, custom
non-comparable nodes, parallel/antiparallel edges, removal cleanup, traversal
orders and early stop, closure strategies, immutable independence, and adapter
agreement. `testdata/guava_graph.json` is a pinned Guava differential oracle.
Matched benchmark and memory records are in `docs/benchmarks/generic_graph/`.
