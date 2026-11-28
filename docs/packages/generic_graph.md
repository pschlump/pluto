# generic_graph

## Data model and complexity

`Graph[N]` stores unique nodes in a stable-ID index (`indexedSet`). Internal
IDs are never reused, so removal cannot make stale adjacency history live again.
Nodes and edges retain isolated values and insertion order. SortedOrder sorts
snapshot results with Config.Less; Unordered makes no order guarantee (it
currently returns insertion order — do not rely on it).

### Storage layout (2026-11-27 rewrite) — invariants to keep

- **Node index.** The natural constructors (`NewGraph`, `NewValueGraph`,
  `NewNetwork`, the Ordered variants) set the unexported `Config.native` /
  `NetworkConfig.nativeEdge`, which builds the index on a Go `map[N]int`
  captured by closures (`nativeIndex`) — no interface boxing. Func
  configurations leave it nil and use the Equal/Hash-driven `hash_grow` table.
  The two paths must keep identical semantics: the natural Equal is `==`, which
  is exactly map key equality (NaN never matches — every `AddNode(NaN)` adds a
  node that cannot be found again; ±0 are one node). `TestNativeIndexEquality`
  pins this and passes on the pre-rewrite code. Transforms reuse `g.config`, so
  they inherit the native index.
- **Adjacency.** `Graph.adj[id]` is parallel to `nodes.entries` (removed nodes
  keep an empty entry). `out` and `in` are EXACT (no stale IDs) and in
  edge-insertion order; removals compact them. `Degree`/`OutDegree`/`InDegree`
  are their lengths, so they must stay exact. An **undirected** graph keeps only
  `out`: the old separate predecessor list was provably the identical sequence
  (each connect appended to `out[x]` and `in[x]` together), so `inList` returns
  `out`. Never populate `in` for undirected graphs.
- **There is no edge map.** Edge membership is `find(u, v)` on `out[u]`: a
  linear scan while `len(out) <= indexThreshold` (16), otherwise a per-node
  open-addressing `index` ([]int32, power-of-two, load <= 1/2) holding
  position+1 into `out`, keyed by the neighbor stored there. Because positions
  shift when `out` is compacted, `removeOutAt` REBUILDS the index (O(degree),
  which the compaction already costs); `pushOut` inserts or regrows. An
  undirected edge appears in both endpoints' `out`, so lookup needs no
  reverse-orientation probe; the stored orientation lives only in `edgeOrder`.
  `edgeCount` is a counter, not a map length.
- **Value slots.** ValueGraph sets `graph.valued`; then `Graph.valueSlots[id]`
  is parallel to `adj[id].out` (an undirected edge carries the same slot in
  both lists) and `ValueGraph.values[slot]` holds the value. Slots are recycled
  through `freeSlots`; freed values are zeroed. Anything that adds edges to a
  valued graph must go through `addEdgeIDs` and store the value with `setSlot`
  — e.g. `ValueGraph.InducedSubgraph` builds a fresh ValueGraph rather than
  wrapping an unvalued `Graph.InducedSubgraph` result.
- **Allocation shape.** Lists grow through `idArena.push`, which carves windows
  of 4, 8 and 16 IDs from shared 2048-ID chunks (three-index slices capped at
  their own end, so windows never overlap); beyond 16 they use ordinary append.
  `edgeOrder` is an `edgeLog`: fixed 2048-entry blocks (only the first block
  starts small), every block but the last full, so appends never recopy the
  whole list. `Copy` packs all lists (and all indexes) into one backing array
  each, again capped per window. Breaking any cap lets an append in one list
  overwrite its neighbor — the mutation tests check this.
- **Complexities.** AddNode/AddEdge expected O(1) amortized; HasEdge/EdgeValue
  expected O(1) (scan of at most 16, or one index probe); RemoveEdge O(E) (the
  edgeLog shift, as before) plus O(degree); RemoveNode O(E) for the edgeLog
  filter plus the degrees of the node's neighbors; Degree O(1).

`Network[N,E]` uses the same `adj` slice (append-only incidence lists of edge
IDs; inactive IDs are skipped by readers, `index` unused) and the arena.

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

Storage tests (`storage_model_test.go`, `storage_extra_test.go`):
`TestGraphStorageDifferential` drives Graph and ValueGraph against a naive
reference of the exact InsertionOrder contract (node order, edge order and
orientation, successor/predecessor order, degrees, incident edges, values),
over native and Func indexes and a hub-heavy shape that crosses
indexThreshold, and continues on `Copy()` results mid-run; the reference was
verified against the pre-rewrite implementation. `TestNetworkStorageFingerprint`
pins FNV fingerprints produced by the pre-rewrite Network. `TestEdgeLogModel`
covers block-boundary deletes, filters and clones.
`TestGraphConstructionAllocations` bounds construction allocations (pre-rewrite
code needed ~10 per node). The `*Expected` benchmarks pass the JMH builders'
capacity hints; the natural constructors cannot (no capacity parameter), so
those benchmarks reach the natural config internally.
