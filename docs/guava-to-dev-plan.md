# Guava-to-Pluto data structures and algorithms development plan

Status: Phases 0 and 1a–1e complete; Phase 2 immutable collections complete for existing bases; Phase 3 and later phases proposed.

Date: 2026-11-22

## 1. Purpose and reference baseline

This plan compares Pluto with Guava's data structures and closely related
algorithms, identifies missing capabilities in existing Pluto packages, and
defines an implementation and performance-comparison program. The goal is
**capability parity where that makes sense in Go**, not a mechanical port of
Java APIs.

References:

- [Guava user guide](https://github.com/google/guava/wiki)
- [New collection types](https://github.com/google/guava/wiki/NewCollectionTypesExplained)
- [Immutable collections](https://github.com/google/guava/wiki/ImmutableCollectionsExplained)
- [Collection utilities](https://github.com/google/guava/wiki/CollectionUtilitiesExplained)
- [Collection helpers](https://github.com/google/guava/wiki/CollectionHelpersExplained)
- [Ranges](https://github.com/google/guava/wiki/RangesExplained)
- [Graphs](https://github.com/google/guava/wiki/GraphsExplained)
- [Caches](https://github.com/google/guava/wiki/CachesExplained)
- [Hashing](https://github.com/google/guava/wiki/HashingExplained)
- [Ordering](https://github.com/google/guava/wiki/OrderingExplained)
- [Math](https://github.com/google/guava/wiki/MathExplained) and
  [statistics](https://github.com/google/guava/wiki/StatsExplained)
- [Primitives](https://github.com/google/guava/wiki/PrimitivesExplained)
- Local source and test reference: `note/guava/`, cloned from
  `https://github.com/google/guava.git`, commit
  `36e3995499d6c3b74b1207d362fa9e19915d2589` (`master` when cloned).
  The `note*` .gitignore rule keeps this checkout local; ordinary Pluto builds
  and tests must not require it.

The source checkout adds important public types not prominent in the wiki:
`EvictingQueue`, `MinMaxPriorityQueue`, `Interner`, immutable primitive arrays,
and the full graph/hash/math surfaces. Package behavior should be checked
against source Javadoc and Guava tests, not inferred only from wiki examples.

## 2. Executive summary

Pluto is already broader than Guava in classic algorithms: balanced trees,
tries, many priority queues, shortest paths, MST, flow/matching, computational
geometry, compression, disk-backed indexes, and probabilistic cardinality are
all present. Those are **not** reasons to port Guava implementations.

The largest genuine gaps are:

1. **Core collection families:** multisets, multimaps, bimaps, and tables.
2. **Immutable snapshots:** list/set/map, sorted variants, and immutable forms
   of the new collection families and graphs.
3. **General ranges:** open/closed/unbounded ranges, discrete domains,
   contiguous sets, range sets, and range maps. `interval_tree` covers only
   closed, bounded intervals and overlap/stabbing queries.
4. **Cache behavior:** atomic loading, per-key load coalescing, expiration,
   refresh, removal causes/listeners, bulk loading, and richer load statistics.
5. **Generic graph data model:** arbitrary node values, isolated nodes,
   node/edge removal, valued edges, unique edge objects, immutable graphs, and
   generic lazy traversal. Pluto's integer-vertex algorithm coverage is much
   stronger than Guava's and should be retained.
6. **Hash infrastructure:** streaming hash/funnel abstractions, Guava-compatible
   non-cryptographic hashes, consistent hashing, and a generic Bloom filter.
7. **Smaller algorithmic utilities:** evicting ring-buffer queue, double-ended
   priority queue, comparator composition/top-k, iterator algorithms, checked
   integer arithmetic, statistics, quantiles, correlation, and least squares.

Recommended first delivery sequence: comparison harness -> multiset ->
multimap -> bimap/table -> small queues and existing-set improvements ->
immutable collections -> ranges -> caches -> hashing/Bloom -> generic graphs ->
iterator/ordering/math utilities. Immutable variants and performance fixtures
land with or immediately after their mutable base types.

## 3. Parity rules and scope

### 3.1 Preserve Pluto's design contract

- Use Go generics and value returns; do not box elements in `any` to imitate
  Java collection interfaces.
- Keep the two-constructor convention: natural `cmp.Ordered`/`comparable`
  construction plus a `...Func` form for custom comparison or equality/hash.
- Keep `(T, bool)`, `error`, and documented programmer-error panics instead of
  reproducing Java null and exception APIs.
- Mutable containers that are useful across goroutines get `_ts` twins.
  Immutable values need no twin and must be safe for concurrent reads.
- Iterators use `iter.Seq`/`iter.Seq2`; snapshot rules for `_ts` types and
  early-stop behavior must be documented.
- Add no production dependency. Java, Maven, JMH, and Guava are test/benchmark
  tools only.
- Do not change existing serialized formats or existing method semantics for
  the sake of Guava compatibility. Add new APIs or new packages instead.

### 3.2 Do not confuse similar names with equivalent capabilities

- `bag` is add-only, insertion-ordered, and has no count/removal operations;
  it is not a Guava `Multiset`.
- `hash_tab*` stores unique elements, not general key/value associations;
  Go's built-in `map` is the mutable hash-map baseline.
- `hash_tab_dll` is not a linked-hash map: its linked lists are buckets, not a
  global insertion order.
- `interval_tree` is an overlap index, not a `RangeSet` or `RangeMap`.
- `lru`, `lfu`, and `tiny_lfu` are eviction structures, not loading caches.
- `suffix_array` and immutable query objects are specialized immutable values,
  not a general immutable-collection family.
- Guava's `Graphs.transitiveClosure` returns a graph snapshot; Pluto's
  `digraph.TransitiveClosure` is a reachability query matrix. Both are useful,
  but they are different interfaces.

### 3.3 Views and Java-specific features

Guava's live, write-through collection views are functional capabilities, not
merely syntax. Initial Go APIs may return snapshots for safety, but the plan
must then add explicit view types where live behavior matters. A Go `map` or
slice returned by value cannot provide a safe write-through view.

Do **not** port features whose only purpose is to compensate for Java:

- primitive/wrapper boxing and primitive-array `asList` adapters;
- null ordering, `Optional`-style null avoidance, precondition helpers,
  `Object` methods, throwable utilities, and reflection;
- forwarding decorators and abstract iterator superclasses;
- enum-specific maps/sets/multisets/bimaps;
- `ClassToInstanceMap`, which would require runtime type keys and `any`
  values without a compelling type-safe Go equivalent;
- Java serialization, Java I/O wrappers, `ListenableFuture`, `Service`, and
  concurrency utilities already addressed by Go contexts/channels/stdlib;
- weak/soft-cache semantics in the first release. Evaluate Go's available
  weak-pointer support separately; never claim deterministic equivalence to
  Java soft references.

EventBus, string joining/splitting/matching, escaping, networking, and general
I/O are outside this data-structure/algorithm plan. Go's `strings`, `bytes`,
`regexp`, `encoding`, `io`, and `net` packages cover much of that territory.

## 4. Coverage map

| Guava capability                                                   | Pluto today                                                               | Disposition                                                     |
|--------------------------------------------------------------------|---------------------------------------------------------------------------|-----------------------------------------------------------------|
| Lists, stacks, queues, deques                                      | `dll`, `sll`, `stack`, `queue`, `dqueue`, `quicklist`, twins              | Covered; add bounded eviction separately                        |
| Hash and sorted sets                                               | `hash_set`, `tree_set`, twins                                             | Covered; add non-destructive/lazy algebra and richer navigation |
| Mutable hash/sorted maps                                           | Go `map`; ordered key/value `binary_search_st`, disk stores               | Mostly covered by Go; immutable maps remain missing             |
| `HashMultiset`, `TreeMultiset`, concurrent and immutable multisets | None; `bag` is not equivalent                                             | **Build**                                                       |
| List/set/sorted multimaps, immutable multimaps                     | None                                                                      | **Build**                                                       |
| `HashBiMap`, immutable bimap, live inverse                         | None                                                                      | **Build**                                                       |
| Hash/tree/dense/immutable tables                                   | None                                                                      | **Build**                                                       |
| Immutable list/set/sorted set/map/sorted map                       | None                                                                      | **Build**                                                       |
| `Range`, `DiscreteDomain`, `ContiguousSet`                         | Closed `Interval` only                                                    | **Build general model**                                         |
| `RangeSet`, `RangeMap`, immutable forms                            | None                                                                      | **Build**                                                       |
| `EvictingQueue`                                                    | Manual eviction with existing queues                                      | **Build ring buffer + twin**                                    |
| `MinMaxPriorityQueue`                                              | Separate min/max or heap structures only                                  | **Build min-max heap + twin**                                   |
| Lazy iterable/iterator algorithms                                  | Partial coverage through `iter.Seq`, `slices`, `maps`, `g_lib`            | **Build focused `iter_util`**                                   |
| Comparator composition, top-k, sorted copies                       | Stored comparators, sorts, min/max                                        | **Build `ordering`**                                            |
| Set Cartesian product/power set, map difference/index              | Set algebra exists; these utilities do not                                | **Build utilities**                                             |
| Generic `Graph`, `ValueGraph`, `Network`                           | Integer `graph`/`digraph`; weighted int graphs                            | **Build generic model**                                         |
| Generic graph traverser and graph views                            | Many int-vertex algorithms, but no generic lazy traverser                 | **Build and adapt to existing algorithms**                      |
| Graph algorithm breadth                                            | DFS/BFS, SCC, closure, Euler, shortest path, MST, flow, matching, min-cut | Already exceeds Guava; no port needed                           |
| Loading/expiration/refresh cache                                   | LRU/LFU/W-TinyLFU eviction, weights, stats; no loading/TTL/refresh        | **Build cache layer**                                           |
| Bloom filter                                                       | `bloom` + twin, merge, clone, approximate count                           | Add FPP/compatibility; build generic funnel variant             |
| Hash functions/funnels/consistent hash                             | Internal table hashes, Bloom's frozen hashes, CRC                         | **Build public hashing package**                                |
| Online/paired statistics, quantiles                                | None                                                                      | **Build**                                                       |
| Checked/saturating arithmetic and rounded integer operations       | Small `g_lib` numeric surface                                             | **Build focused math utilities**                                |
| Primitive conversion/unsigned operations                           | Native Go types, `strconv`, `encoding/binary`, `math/bits`                | No parity project required                                      |
| Strong/weak interning                                              | None                                                                      | Optional later capability                                       |

## 5. Phase 0 — contracts, golden oracles, and comparison harness

Estimated effort: 2–3 days.

1. Add `internal/guava_compare/` for committed Go benchmark drivers, Java/JMH
   sources, corpus generation, and result normalization. Keep it out of normal
   package tests and production imports.
2. Pin the Java source to the commit above. A deliberate Guava update must
   record the new commit and regenerated oracle/benchmark baseline.
3. Build a small Java command-line oracle that consumes documented JSON or
   binary operation traces and emits canonical results. Generate golden files
   into each package's `testdata/`; normal `go test ./...` reads files only and
   never invokes Java.
4. Reuse the matched-workload approach in
   `tiny_lfu/testdata/CaffeineBenchmark.java`: Go `testing.B` on one side, JMH
   on the other, identical generated corpora and operation streams.
5. Add an opt-in `make guava-bench` target only when the harness exists. It may
   check for Java/Maven and print an actionable error; `make build test vet
   race` must work without them.
6. Record machine, OS/architecture, Go and JDK versions, Guava commit, JMH
   settings, capacity, input distribution, and random seed with every result.
   Report throughput/latency, allocations, and memory separately; do not reduce
   correctness or hit-rate differences to a single speed ratio.

Exit criterion: one trivial end-to-end fixture proves that the same trace can
be executed by Java and Go and that benchmark output can be normalized without
becoming a default test dependency.

## 6. Phase 1 — missing core collection families

### 6.1 `multiset` and `multiset_ts` — priority P0

Estimated effort: 4–6 days including immutable integration later.

Implement `HashMultiset[T]` and `TreeMultiset[T]` in one family package, with a
thread-safe hash multiset twin. Add `LinkedHashMultiset` only if stable first
insertion order is needed after the base types are proven.

Core contract:

- `Add(value, occurrences)`, `Remove(value, occurrences)`,
  `SetCount(value, count)`, `Count(value)`, `Len()` (all occurrences),
  `DistinctLen()`, `DeleteAll(value)`, and `Clear`.
- Zero counts remove the element; negative counts and count overflow are
  programmer errors. Specify whether sizes/counts are `int` or `uint64` and use
  that choice consistently.
- `Entries()` yields distinct `(element, count)` pairs; `Elements()` yields
  distinct elements; `All()` yields every occurrence. Memory is O(distinct).
- Batch operations, content equality, clone, JSON, highest-count-first copy,
  and `ContainsOccurrences`.
- Count-aware `Union` (max), `Sum` (add), `Intersection` (min), and `Minus`
  (saturating subtraction), distinct from `hash_set` semantics.
- Sorted variant: min/max, floor/ceil, ascending/descending iteration, and
  count-preserving sub/head/tail ranges with open/closed bounds.

Prefer composition over `hash_grow`/`avl_tree` if entry-shaped values preserve
all required complexity. Otherwise implement a dedicated node/table core;
do not silently turn sorted range operations into O(n) scans.

Tests: fixed Guava-derived vectors plus seeded model tests against
`map[T]int`; zero-count key removal, duplicate iteration length, overflow,
alias-safe algebra, sorted bounds, JSON round-trip, and twin atomicity/race
tests. Differential traces must distinguish total size from distinct size.

Benchmarks: Guava `HashMultiset`/`TreeMultiset` and a Go `map[T]int` baseline;
add/remove/set-count, hit/miss count, distinct-entry iteration, full occurrence
iteration, and algebra at several duplicate ratios. Compare memory by distinct
key count and total occurrence count.

### 6.2 `multimap` and `multimap_ts` — priority P0

Estimated effort: 5–8 days.

Provide `ListMultimap[K,V]`, `SetMultimap[K,V]`, and `SortedSetMultimap[K,V]`.
Start with natural comparable/ordered forms and custom function constructors;
do not create one package per backing implementation.

Core contract:

- `Put`, `PutAll`, `Get`, `Remove(key,value)`, `RemoveAll(key)`,
  `ReplaceValues`, `ContainsKey`, `ContainsEntry`, `ContainsValue`, `Len`
  (associations), and `KeyLen` (distinct keys).
- A key never remains mapped to an empty value collection. List values preserve
  duplicates and insertion order; set values collapse duplicates; sorted set
  values have deterministic comparator order.
- `Entries`, `Keys` with multiplicity, `Values`, `KeySet`, and `AsMap` views.
- Utilities `Index`, `InvertFrom`, and `ForMap`; integration with multiset and
  immutable multimap constructors.

Release 1 may make `Get` and `AsMap` snapshots. A following milestone must add
explicit `ValuesView`/map-view objects with `Get`/`Put`/`Delete`/iteration if
live Guava-style write-through views are required. Document the difference
until then; never return an ordinary mutable map and call it a view.

Tests: no-empty-key invariant after every mutation, list versus set duplicates,
replacement return values, flattened value counts, inversion, key multiplicity,
and custom equality/comparison. Twin tests cover batch atomicity, opposite-way
operations without nested locks, snapshots, and `-race`.

Benchmarks: Guava `ArrayListMultimap`, `HashMultimap`, and `TreeMultimap` versus
their Go counterparts and `map[K][]V`/`map[K]map[V]struct{}` baselines. Measure
put/get/remove, duplicate-heavy and many-values-per-key shapes, iteration, and
bytes per association.

### 6.3 `bimap` and `bimap_ts` — priority P0

Estimated effort: 3–4 days.

Implement two synchronized indexes behind `HashBiMap[K,V]`:

- `Put` rejects an already-used value with an error; `ForcePut` removes the
  conflicting old association; updating an existing key maintains both maps.
- `GetKey`/`GetValue`, deletion from either direction, `Len`, iteration, clone,
  equality, and JSON.
- `Inverse()` returns a coupled `HashBiMap[V,K]` view sharing one core, not an
  independently mutable copy. The `_ts` inverse also shares one lock; do not
  lock forward and inverse cores separately.

Immutable and insertion-ordered variants come after the mutable invariants are
stable. Enum variants are deliberately omitted.

Tests: every conflicting put/force-put permutation, inverse mutations visible
in both directions, no stale reverse entries, inverse-of-inverse identity,
clone independence, and concurrent inverse operations under `-race`.

Benchmarks: Guava `HashBiMap` versus two manually synchronized Go maps;
forward/reverse lookup, put/update/conflict handling, inverse iteration, and
memory overhead.

### 6.4 `table` and `table_ts` — priority P1

Estimated effort: 4–6 days.

Provide sparse `HashTable[R,C,V]`, ordered `TreeTable[R,C,V]`, and dense
`ArrayTable[R,C,V]` with a common semantic contract but concrete Go types.

- `Cell[R,C,V]`, `Put`, `Get`, `Delete`, `Contains`, `ContainsRow`,
  `ContainsColumn`, `Len`, `Rows`, `Columns`, `Cells`, `Row`, `Column`,
  `RowKeys`, and `ColumnKeys`.
- Empty rows/columns disappear from sparse tables. Column operations may be
  slower than row operations in a row-major implementation; document and measure
  rather than hiding the cost.
- `Transpose` and immutable conversion. `ArrayTable` fixes complete row/column
  universes at construction, distinguishes absent cells from zero `V`, and
  offers O(1) indexed access.
- Start with snapshot rows/columns; add explicit live row/column/cell views only
  as a separately tested milestone.

Tests: sparse model against nested maps, dense model against a matrix, empty
view cleanup, transpose round-trip, custom ordering, zero values, JSON, and
twin snapshot/atomicity behavior.

Benchmarks: Guava `HashBasedTable`, `TreeBasedTable`, and `ArrayTable`; sparse
and dense shapes, row/column access, transpose, and memory. Include nested Go
maps and a flat `[]V` matrix as honest baselines.

### 6.5 Small missing structures — priority P1

Estimated effort: 3–4 days.

**`evicting_queue` + `_ts`**

- Fixed-capacity ring buffer; `Push` always succeeds and evicts the oldest
  element when full. Capacity zero accepts and immediately discards.
- `RemainingCapacity`, O(1) push/pop/peek, bulk-push fast path retaining only
  the newest `capacity` inputs, logical-order iteration, clone/equality/JSON.
- Zero stale slots after pop/eviction so references can be collected.
- Model against a slice queue; test wraparound, zero/one capacity, and bulk
  input larger than capacity. Benchmark against Guava `EvictingQueue` and Pluto
  `dqueue` plus manual eviction.

**`min_max_pq` + `_ts`**

- Single-array min-max heap with O(1) `PeekMin`/`PeekMax` and O(log n)
  insertion/removal at either end.
- Optional maximum size evicts the greatest element, possibly the new element.
- Preserve comparator, clone, equality, iteration, and JSON conventions; add
  level-aware min/max invariant checks.
- Model every operation against a sorted multiset; exercise bounded inserts,
  equal keys, both removal ends, and heap levels. Benchmark Guava
  `MinMaxPriorityQueue`, Pluto `heap`, and a two-heap alternative. Include the
  documented warning that one-ended workloads usually favor a normal heap.

## 7. Phase 2 — immutable collection family

Estimated effort: 8–12 days after the mutable families.

Create one `immutable` package for closely related `List[T]`, `Set[T]`,
`Map[K,V]`, `SortedSet[T]`, `SortedMap[K,V]`, `Multiset[T]`, `Multimap[K,V]`,
`BiMap[K,V]`, `Table[R,C,V]`, `RangeSet[T]`, and `RangeMap[K,V]` types as the
corresponding bases become available. Separate packages are acceptable if the
shared builders/conversion helpers create import cycles; decide that structure
before publishing constructors.

Required behavior:

- Construction defensively copies mutable input; no caller can mutate stored
  arrays/maps after construction. Iterators never expose mutable internals.
- Lists provide O(1) `At`, length, sub-list, and forward/reverse iteration.
- Hash sets/maps provide expected O(1) membership/lookup. Preserve first
  occurrence/entry construction order where Guava does; sorted variants use a
  documented comparator and deterministic order.
- Sorted variants provide rank/random access, lower/floor/ceil/higher, and
  head/tail/sub-range selection. Return safe immutable subslices/views without
  retaining unexpectedly large backing arrays when a copy is cheaper overall.
- Duplicate map keys and duplicate bimap values fail construction; multiset and
  list-multimap duplicates follow their mutable semantics.
- Builders support incremental construction followed by one snapshot. Reusing a
  builder must not mutate values already built.
- `CopyOf` may return an existing immutable value unchanged; conversions that
  change equality/order semantics must copy.
- JSON marshal is useful; prefer parse/build functions over public
  `UnmarshalJSON` methods that make an already-published immutable value
  mutable. Document this deliberate exception to mutable-container convention.
- No `_ts` twins. Immutable means the collection structure is immutable, not
  that referenced mutable payloads become deeply immutable.

Tests: attempted mutation of every source after construction, builder reuse,
iterator safety, concurrent reads under `-race`, duplicate failures, sorted
navigation, conversion semantics, and memory-retention tests for subviews.
Use Guava immutable-collection test cases as behavioral references.

Benchmarks: construction/copy-of, lookup/contains, rank and subrange, sequential
iteration, and allocation/retained memory against Guava immutable types, Go
maps/slices, and Pluto mutable equivalents. Use JOL on Java and construction
allocation plus GC-stable heap measurements on Go; label the memory methods as
different if they are not directly comparable.

## 8. Phase 3 — ranges and ordered-navigation enhancements

Estimated effort: 8–12 days.

### 8.1 `ranges`

Implement immutable `Range[T]` with natural and custom-comparator construction:

- `Open`, `Closed`, `ClosedOpen`, `OpenClosed`, `GreaterThan`, `AtLeast`,
  `LessThan`, `AtMost`, and `All`.
- Bound introspection, `Contains`, `ContainsAll`, `IsEmpty`, `Encloses`,
  `IsConnected`, `Intersection`, and `Span`.
- Exactly preserve endpoint rules: reversed bounds invalid; `[a,a]` singleton;
  `[a,a)` and `(a,a]` empty; `(a,a)` invalid. Unbounded endpoints are explicit,
  not represented by an arbitrary zero value.
- `DiscreteDomain[T]` as a function/value bundle (`Next`, `Prev`, `Distance`,
  optional min/max), with checked signed and unsigned integer domains. Detect
  successor/predecessor and distance overflow.
- `Canonical(domain)` and lazy `ContiguousSet` with iteration, rank, and size
  where finite. Do not materialize an entire contiguous range.

Use exhaustive small-endpoint vectors and Guava differential traces, including
empty ranges, touching open/closed endpoints, infinities where applicable, and
integer limits.

### 8.2 `range_set` / `range_map`, with `_ts` for mutable variants

- `RangeSet`: add/remove, connected-range coalescing, empty-range no-op,
  `Contains`, `RangeContaining`, `Encloses`, `Span`, `Complement`,
  `SubRangeSet`, and ordered range iteration.
- Canonicalize with a discrete domain before merging ranges such as
  `[1,10]` and `[11,15)`; do not pretend those are connected for all ordered
  types.
- `RangeMap`: put/remove split existing mappings at exact boundaries; adjacent
  equal values are **not** coalesced; support `Get`, `RangeContaining`,
  `SubRangeMap`, and ordered entries.
- Add immutable conversions in `immutable`; reuse range logic, not interval
  overlap semantics.
- Initially return subrange snapshots. Add live complement/subrange views only
  with explicit view types and mutation tests.

Model tests use finite integer universes so every range can be compared to an
exact point set/map. Include split/merge torture traces, unbounded complements,
discrete canonicalization, and no-coalescing checks. Benchmark Guava
`TreeRangeSet`/`TreeRangeMap` for random and clustered ranges, point queries,
insert/remove splits, and iteration.

### 8.3 Upgrade existing ordered structures and `interval_tree`

- Keep `interval_tree` closed-interval API unchanged. Add conversion/adapters
  for fully closed bounded `ranges.Range`, plus `InsertRange`/`OverlapsRange`
  helpers that reject unsupported open/unbounded inputs. Do not claim that this
  provides `RangeSet` or `RangeMap`.
- Add `Lower`/`Higher` and open/closed/unbounded range queries to `tree_set`,
  `skip_list`, and `binary_search_st` (+twins). Keep existing inclusive `Range`
  methods unchanged and add `RangeOf(ranges.Range)` or clearly named bounded
  variants.
- Replace `tree_set.Range`'s current filtered O(n) walk with positioned AVL
  descent so it becomes O(log n + results). This likely requires an internal
  lower-bound iterator in `avl_tree`/`avl_tree_ts`; preserve AVL invariants and
  snapshot behavior.
- Add non-destructive set result constructors and a later lazy `SetView` for
  union/intersection/difference/symmetric difference. Existing destination-form
  algebra remains source-compatible. Add lazy Cartesian-product and power-set
  iterators with early-stop tests and documented exponential output size.
- Add `map_util`: `UniqueIndex`, `MapDifference` (common, differing, left-only,
  right-only), and function-based value equality for non-comparable `V`.
  `UniqueIndex` should produce an immutable map once Phase 2 exists.

Benchmark range queries before/after the AVL change and against Guava sorted
sets. Verify that the improvement is O(log n + k), not merely a constant-factor
change.

## 9. Phase 4 — Guava-style loading and expiring caches

Estimated effort: 8–12 days.

Build a new `cache` package rather than changing `lru`/`tiny_lfu` method
semantics. Reuse an eviction core only if its hooks can support the contracts;
otherwise keep policy and loading separate. A configurable `Cache[K,V]` and
`LoadingCache[K,V]` should support:

- `GetIfPresent`/`Peek` without loading, direct `Put`, individual/bulk/full
  invalidation, `CleanUp`, current-size iteration/snapshot, and stats.
- `Get(ctx,key)` with a `Loader func(context.Context,K) (V,error)` and
  `GetWith(ctx,key,loader)`. Coalesce concurrent misses for the same key so the
  loader runs once; unrelated keys load concurrently. Never hold the cache
  policy lock while running caller code.
- `GetAll` and optional bulk loader. Define partial-success behavior and retain
  existing values on failed refresh.
- Maximum size and maximum weight (static weight at write), expire-after-write,
  expire-after-access, and refresh-after-write. Inject a clock/ticker function;
  no sleeps in correctness tests.
- Refresh returns the old value while replacement is in progress; support an
  explicit executor for asynchronous reload. Context cancellation applies to
  the caller/load according to a documented coalescing rule.
- Removal notifications with causes: explicit, replaced, expired, size, and
  loader-error/invalidated as applicable. Run listeners outside internal locks;
  offer asynchronous listener dispatch without making it the default.
- Stats: hits, misses, loads, load successes/failures, total load time, and
  evictions, in addition to the existing TinyLFU counters where reused.
- Optional `PutIfAbsent`/`Replace`/`DeleteValue` atomic map-style operations.
  Loading and these operations must have documented interaction.

Do not change Bloom-like serialization or cache disk formats. Do not add a
background goroutine per cache by default; Guava-style operation-driven cleanup
plus explicit `CleanUp` is the baseline. Weak/soft references are deferred.

Tests: fake-clock expiration boundaries, access-versus-write timestamps,
single-flight under contention, loader panic/error/cancellation, recursive
loader behavior, refresh keeps old value, bulk loads, every removal cause,
listener re-entry/no deadlock, weight updates, and race/stress suites. Compare
observable traces with Guava where policies match; where Guava and Pluto use
different eviction policies, compare loading/expiration behavior separately
from victim choice.

Benchmarks: matched Guava `CacheBuilder`/`LocalCache` JMH and Go drivers for
single-thread and 1/8/N-thread hits, updates, evicting writes, miss loading,
coalesced misses, expiration, and refresh. Include Zipf, scan, and phase-change
workloads; hit rate and load count are correctness/performance outputs. Compare
against both Pluto `lru_ts` and `tiny_lfu_ts`, and retain the existing Caffeine
benchmarks as a separate baseline rather than substituting Guava for Caffeine.

## 10. Phase 5 — hashing and Bloom enhancements

Estimated effort: 6–10 days.

### 10.1 `hashing`

Implement a small streaming abstraction modeled by capability, not Java syntax:

- Immutable `HashCode` bytes with explicit endianness, integer/byte accessors,
  equality, and bounded copy.
- `HashFunction`/`Hasher` or equivalent concrete interfaces with streaming
  primitive and byte/string writes. A generic `Funnel[T]` encodes a `T` into a
  primitive sink without storing `T` as `any`.
- Guava-compatible Murmur3-32, Murmur3-128, SipHash-2-4,
  FarmHashFingerprint64, and Fingerprint2011, with known-answer vectors.
- Standard-library wrappers for CRC32/CRC32C/Adler32 and MD5/SHA families only
  where useful; label MD5/SHA-1 as interoperability, not secure new design.
- `CombineOrdered`, `CombineUnordered`, and jump-consistent-hash bucket
  selection. Validate bucket count and pin bucket-boundary vectors.

Byte-exact Guava golden tests must cover chunked versus unchunked writes,
lengths around block boundaries, signed/unsigned bytes, seeds, empty input, and
non-ASCII strings. Funnel tests must show that callers can add delimiters or
length prefixes to avoid concatenation ambiguity.

Benchmark hash throughput by input size and streaming chunk size against Guava
and Go stdlib where the same algorithm exists. For consistent hashing, measure
speed but also verify minimal-remapping and distribution properties.

### 10.2 Existing `bloom` and a generic Guava-compatible variant

Add to existing `bloom`/`bloom_ts` without changing its hashes or serialized
format:

- `ExpectedFPP() = Saturation()^HashCount()` with full-saturation behavior
  documented.
- `Compatible(other)` for shape/strategy checks before `Merge`; `Count`
  already supplies approximate element count and `Clone` already supplies copy.
- Golden tests comparing formulas and compatibility, with no change to the
  frozen MurmurHash2/SuperFastHash probe vectors.

Add a separate generic `bloom_filter` (plain plus `_ts`, or atomics if that
proves simpler and faster) using `hashing.Funnel[T]` and Guava's Murmur3-based
strategy. Support create with expected insertions/FPP, put/might-contain,
copy, merge/compatibility, expected FPP, approximate count, and stream
serialization. Its wire format is new and versioned; optionally implement
Guava `writeTo`/`readFrom` compatibility as an explicit codec. Never silently
reinterpret existing `bloom.Bytes()` as this format.

Differential tests compare bit arrays and membership for shared funnels and
measure FPP over disjoint non-member sets. Benchmarks compare add, hit/miss,
merge, serialization, allocations, and accuracy at/above design load against
both Guava and existing Pluto `bloom`.

## 11. Phase 6 — generic graph model and traversal

Estimated effort: 12–18 days; highest design risk, so do not block Phases 1–5
on it.

Create `generic_graph` with `Graph[N]`, `ValueGraph[N,V]`, and `Network[N,E]`
as concrete related types or minimal interfaces only where shared algorithms
benefit. Nodes must be unique; `ValueGraph` edge values need not be unique;
`Network` edges are first-class unique values and support parallel edges.

Configuration must cover directed/undirected, self-loops allowed/disallowed,
parallel edges where meaningful, expected capacity, and unordered/insertion/
sorted iteration. The implementation must support:

- Add/remove nodes and edges, isolated nodes, node/edge existence, node/edge
  counts, adjacency, predecessors/successors, incident edges, degree/in/out
  degree, endpoint pairs, and edges connecting two nodes.
- Mutation errors that identify unknown nodes, duplicate edges, disallowed
  loops/parallel edges, and inconsistent endpoint operations.
- `Traverser`-like lazy BFS, DFS preorder, and DFS postorder over either a graph
  or `func(N) iter.Seq[N]`; graph traversal deduplicates, tree traversal avoids
  the visited set and documents its unique-path precondition.
- `ReachableNodes`, graph-producing transitive closure with explicit self-loop
  strategy, transpose, induced subgraph, and copy. Provide live transpose views
  only after snapshot/copy behavior is correct.
- Immutable graph/value-graph/network snapshots after Phase 2 principles;
  concurrent reads need no lock.
- Adapters that assign dense temporary integer IDs and call existing
  `graph`/`digraph`/`dijkstra` algorithms, then map results back to `N`. Do not
  rewrite all existing algorithms before proving adapter overhead.

Also add cheap traversal-order query APIs to existing `graph`/`digraph` and
twins where useful, but keep their optimized `[][]int` representation and
existing query snapshots. Node removal in generic graph must not be used as a
reason to destabilize those classic packages.

Tests: operation models against adjacency maps, isolated-node preservation,
self-loop degree rules, parallel/antiparallel edges, removal cleanup, stable
iteration configuration, traversal orders over known graphs, closure self-loop
strategies, immutable independence, and adapter agreement with existing int
algorithms. Generic `_ts` mutable wrappers, if provided, snapshot traversals
and never hold locks during caller successor functions.

Benchmarks: Guava mutable graph/value graph/network versus generic Pluto for
node/edge insertion, lookup, removal, adjacency, traversal, and memory across
sparse/dense graphs. Separately measure generic-to-int adapter plus Pluto
algorithm time against running Guava traversal/utilities; report adapter cost
rather than hiding it in the algorithm number.

## 12. Phase 7 — iterator, ordering, and collection algorithms

Estimated effort: 5–8 days.

### 12.1 `iter_util`

Implement focused lazy functions over `iter.Seq`/`iter.Seq2`:

- `Concat`, `Filter`, `Map`, `Take`, `Skip`, `Partition`, `Frequency`,
  `ElementsEqual`, `First`, `Last`, `Only`, `At`, `IsEmpty`, and `Len`.
- A pull-based `Peeking` cursor with `Next`/`Peek` and idempotent `Peek`.
- Pair/key/value adapters and conversion to/from slices where materialization
  is explicit.

Every lazy function must propagate early termination to its input and avoid
retaining consumed elements. Tests include empty/singleton/infinite-until-take
sequences, panicking producers that must not be over-consumed, and partition
boundaries. Go stdlib baselines are more important than Java for these APIs;
Guava benchmarks are optional for concat/filter/partition/peek only.

### 12.2 `ordering`

Use a function-backed `Ordering[T]` value:

- `Natural`, `FromFunc`, `Reverse`, `Then`, `ByKey`, `Lexicographical`, and
  explicit all-equal ordering.
- `Min`/`Max`, `IsOrdered`/`IsStrictlyOrdered`, stable `SortedCopy`, and
  `LeastOf`/`GreatestOf` using a bounded heap in O(n log k), not a full sort.
- Nil-first/nil-last only as pointer-specific helpers if needed, not a generic
  pretense that every `T` can be nil.

Tests compare against `cmp.Compare`/`slices.SortStableFunc`, test tie stability,
k=0/k>n, equal elements, lexicographic prefixes, and comparator composition.
Benchmark Guava `Ordering` and Go full-sort and `g_lib` baselines, varying k and
input order.

## 13. Phase 8 — math and statistics

Estimated effort: 6–10 days.

### 13.1 `math_util` (or a deliberately scoped `g_lib` addition)

Prioritize operations with correctness value over duplicating `math/bits`:

- Checked and saturating signed/unsigned add/subtract/multiply/pow; checked and
  saturating conversions.
- Divide/log2/log10/sqrt with explicit rounding modes; nonnegative modulus,
  gcd, factorial, binomial, power-of-two checks, overflow-safe mean, and
  primality testing for standard integer widths.
- `math/big` variants only for operations not already well covered by `Int`.
- Document NaN, infinity, divide-by-zero, negative inputs, and overflow behavior.

Exhaustive boundary tests around every integer limit, independent big-integer
or 128-bit oracles, and Guava golden traces are mandatory. Benchmark only
non-trivial operations against Guava and straightforward Go implementations.

### 13.2 `stats`

- Mergeable online `Accumulator` and immutable `Stats`: count, sum, mean, min,
  max, population/sample variance and standard deviation.
- `PairedAccumulator`: Pearson correlation and least-squares fit;
  `LinearTransformation` with transform/inverse/vertical/horizontal semantics.
- Exact Type-7 quantiles, median, percentiles, and multiple indexes in one
  selection pass; copying and in-place variants.
- Use numerically stable online updates and define non-finite/empty/sample-size
  behavior. A selection algorithm should avoid an easy adversarial O(n^2)
  worst case if that does not change results.

Tests: known datasets, shifted/scaled large values, accumulator merge versus
one-pass, horizontal/vertical/constant paired data, NaN/infinities, quantile
interpolation, duplicate indexes, and sort-based quantile oracle. Benchmark
Guava `StatsAccumulator`/`PairedStatsAccumulator`/`Quantiles`, naive Go
calculations, and sort-based quantiles; vary dataset size and number of
quantiles requested together.

## 14. Phase 9 — optional completeness work

Estimated effort: 3–5 days; schedule only with a use case.

- `intern`: strong concurrent canonicalization of comparable values, with
  lookup/deduplication semantics and memory benchmarks against Guava
  `Interner`. Treat weak interning as a separate design because lifetime and
  equality semantics differ.
- Insertion-ordered linked hash multiset/multimap/map variants if deterministic
  insertion order proves important beyond immutable construction order.
- Full live, write-through multimap/table/set views if snapshot/view milestones
  show a real requirement. These are functional work, not cosmetic wrappers.
- Guava-compatible Bloom wire codec and additional graph sorted-order backends
  after interoperability or performance requirements are demonstrated.

## 15. Cross-cutting verification and delivery rules

Every package/family phase includes:

1. Public API contract and complexity table before implementation.
2. Checked examples, fixed vectors, and fixed-seed randomized model tests.
3. Structural invariant checkers for heaps, trees, linked indexes, and graph
   connections; run them after randomized mutations.
4. `_ts` behavioral parity, batch atomicity, snapshot iteration, callback
   no-deadlock, and `go test -race` coverage where a twin exists.
5. Guava differential goldens where semantics correspond. Canonicalization must
   not erase meaningful order, duplicate, empty-view, or self-loop differences.
6. Package `README.adoc`, `docs/packages/<name>.md`, root `README.adoc` package
   index/details, and `docs/packages/twins.md` updates in the same change.
7. `make build test vet race lint`, empty `gofmt -l .`, and `go mod tidy` with
   no new production dependency or root `go.sum` unless deliberately approved.
8. Benchmarks run on the committed implementation, with raw and summarized
   results retained under the benchmark documentation. No performance claim
   based only on asymptotic complexity or a single run.

For direct ports of Guava implementation code, retain Apache-2.0 attribution
and required notices and make the combined licensing explicit. Prefer behavioral
ports and independent Go models where practical. The local clone is a reference,
not code to vendor wholesale.

## 16. Rollout, dependencies, and rough effort

| Order | Deliverable | Depends on | Priority | Estimate |
|---|---|---|---|---|
| 0 | Golden/differential and Go/JMH harness | None | P0 | 2–3 days |
| 1a | Multiset + twin | Phase 0 | P0 | 4–6 days |
| 1b | Multimap + twin | Phase 0; multiset integration | P0 | 5–8 days |
| 1c | Bimap + twin | Phase 0 | P0 | 3–4 days |
| 1d | Table + twin | Phase 0 | P1 | 4–6 days |
| 1e | Evicting queue and min-max PQ | Phase 0 | P1 | 3–4 days |
| 2 | Immutable family and conversions | Relevant mutable bases | P0/P1 | 8–12 days |
| 3 | Ranges, range set/map, ordered upgrades | Phase 2 for immutable forms | P0/P1 | 8–12 days |
| 4 | Loading/expiring cache | Phase 0; existing cache cores | P0 | 8–12 days |
| 5 | Hashing and generic Bloom | Phase 0 | P1 | 6–10 days |
| 6 | Generic graph family and adapters | Phase 2 for immutable forms | P1 | 12–18 days |
| 7 | Iterator/ordering/collection utilities | Existing sets/maps; immutable index | P2 | 5–8 days |
| 8 | Math/statistics/quantiles | Phase 0 | P2 | 6–10 days |
| 9 | Optional interning/live-view completion | Corresponding base types | P3 | 3–5 days |

Rough total: 70–105 focused working days at Pluto's documentation, testing,
race, and benchmarking standard. Phases 1a–1e can be parallelized after the
shared contracts and harness settle. Do not start all immutable variants at
once: prove immutable list/set/map first, then apply the pattern.

### Definition of useful parity

A family is considered delivered only when its semantics and complexity are
documented, model/golden/race tests pass, immutable or `_ts` variants required
by that family exist, and its practical benchmark has either been run or has a
recorded reason why comparison is not meaningful. Capability parity does not
require matching Java names, null behavior, serialization formats, or an
unidiomatic live-view API at the expense of Go safety.
