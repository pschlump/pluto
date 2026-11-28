# linked_hash_map / linked_hash_map_ts

`Map[K,V]` is a hash map with deterministic first-insertion iteration. Updating
a value keeps its position. Deleting and later reinserting a key appends it.
Natural construction uses `==` and `maphash`; `NewFunc` accepts matching
key-equality and hash functions. Stored keys retain the first inserted key value,
as Go map assignment does.

| Operation | Complexity |
|---|---|
| Get / ContainsKey / Put / Delete | expected O(1) |
| Len / IsEmpty | O(1) |
| Entries / Keys / Values | O(n), insertion order |
| Clear / Clone | O(n) |

## Index internals (2026-11-27)

Two index kinds share one insertion-order node list (`head`/`tail`, one heap
node per association; the node list alone defines iteration order):

- **`New` (comparable keys): the native index.** `m.native` holds closures over
  a Go built-in `map[K]*node` created in `newNativeIndex[K comparable]`, because
  `Map[K any, V any]` cannot itself declare a `map[K]` field. Keys pass by value
  through ordinary function values — nothing is boxed into an interface. The
  stored `equal`/`hash` are still built (`Eq()`/`Hash()` return them) but the
  native path never calls them. When `native != nil`, `slots` stays empty.
- **`NewFunc`: a linear-probing table** of `slot{hash, node}` cells, power-of-two
  sized, allocated lazily on the first `Put`, grown by doubling when
  `(length+1)*4 > len(slots)*3` (load ≤ 3/4 — `place` and `lookup` loop until a
  free cell, so the table must never fill). The home cell is
  `(hash*fibMul) >> shift` (Fibonacci mixing so weak caller hashes such as
  identity or `k<<40` still spread). The full caller hash is cached in the cell:
  probes compare it before calling `equal`, and growth never re-calls `hash`.
  Deletion is **backward-shift** (no tombstones): `removeAt` moves a later
  cluster member into the hole unless its home lies cyclically in
  `(hole, next]`. Many keys with one hash form one cluster (O(k) per lookup, the
  same bound as the old chained buckets).

Invariants future editors must keep: `Clone` and both `UnmarshalJSON`s must
preserve the index kind — they go through `empty()` (which calls
`native.fresh()`), and the twin's `UnmarshalJSON` delegates to the plain
method for a constructed inner map. "Constructed" means `hash != nil` (the
zero-value test for `writeContract`, `Clear` and `find`); there is no bucket
map any more. `checkIndex` in `index_test.go` asserts the list/index agreement,
the cached hashes, the load bound and the backward-shift reachability property;
`TestIndexDifferentialModel` drives five index flavors (native, identity, mod-7,
all-zero and high-bit hashes) against a built-in map + order slice and checks
that `Clone`/JSON keep the kind. Mutations of the shift condition, the load
factor and the `fresh()` call all fail it.

Before this, every map — even `New` — was `map[uint64][]*node`: each lookup ran
maphash, then the runtime hashed the uint64 again, then walked a separately
allocated bucket slice, then called `equal`. `Get`+`Put` paid that twice. See
docs/benchmarks/phase9/README.md, 2026-11-27, for the measurements.

`Put` returns the previous value and presence flag. `Delete` returns the removed
value and presence flag. Iterators expose no mutable internal slices. The plain
map must not be mutated during iteration or used concurrently.

The `_ts` twin protects compound operations with one RWMutex. Iterator methods
materialize snapshots under a read lock and release it before caller code runs.
No twin method holds locks belonging to two maps.

JSON uses an insertion-ordered array of `{key,value}` entries, avoiding Go's
restrictions on generic JSON object keys. Decoding validates before replacement
and duplicate keys update their first position.

Tests include a fixed-seed built-in-map model, collisions, custom equality,
update/delete/reinsert order, cloning, JSON failure atomicity, early stop, twin
snapshots, and concurrent access. `Phase9Oracle` generates
`linked_hash_map/testdata/guava_linked_map.json` from `LinkedHashMap`. Matched
Go/JMH put/get and iteration benchmarks are recorded under `docs/benchmarks/phase9/`.
