# immutable

## Construction and shared contract

`immutable` contains defensive, read-only collection snapshots. Constructors
copy caller slices and entries. No method exposes internal mutable slices, maps,
or indexes. Values are safe for concurrent reads without locks, so there is no
`_ts` twin; referenced mutable payloads are not deep-copied.

Natural constructors use `comparable`/`cmp.Ordered`. Func constructors accept
custom equality/hash pairs or comparators. Equal hashes are required for equal
keys. Equality operations require consistent operand equality/comparison
functions. Builders collect input and each `Build` creates an independent value;
later builder reuse cannot mutate earlier builds. `CopyOf...` returns an existing
immutable value unchanged. Conversion functions that change ordering or equality
construct a new value.

Map, sorted-map, and bimap duplicate keys return `ErrDuplicateKey`; duplicate
bimap values return `ErrDuplicateValue`. Set and sorted-set duplicates collapse
to their first occurrence. Multiset counts every occurrence, and Multimap keeps
list values in input order, including duplicates. `table.ImmutableTable` is
reused as `immutable.Table`, with `FreezeHashTable`, `FreezeTreeTable`, and
`FreezeArrayTable` conversions.

`immutable.RangeSet` and `immutable.RangeMap` freeze the Phase 3 mutable bases,
retain discrete-domain behavior, and support safe concurrent reads.

## Types and complexity

| Type | Lookup/access | Selection and iteration |
|---|---|---|
| List | O(1) `At`, `Len` | O(1) `SubList` view when retained; O(n) forward/reverse `All` |
| Set | expected O(1) `Contains` | O(n) first-occurrence `All` |
| Map | expected O(1) `Get` | O(n) first-entry `Entries`, `Keys`, `Values` |
| SortedSet | O(log n) `Contains`, navigation, `Rank`; O(1) `At` | O(log n) head/tail/subrange plus optional retention copy |
| SortedMap | O(log n) `Get`, navigation, `Rank`; O(1) `At` | ordered and reverse entries; O(log n) ranges plus optional copy |
| Multiset | expected O(1) hash `Count`; O(log n) tree-snapshot `Count` | distinct entries/elements or all occurrences |
| Multimap | expected O(1) key index plus O(values) copied `Get` | entries, repeated keys, values, key set, copied AsMap groups |
| BiMap | expected O(1) forward/reverse lookup | ordered entries and O(1) coupled `Inverse` |
| Table | same as `table.ImmutableTable` | copied row/column results and ordered cells |

List and sorted subranges normally share immutable backing arrays. A small
subview of a much larger array is copied instead, avoiding unexpected retention;
that copy costs O(result size). All returned `Slice`, `EntriesSlice`, `Get`, and
`AsMap` slices are independent copies. Iterators propagate early termination.

`Slice` and `All` for heaps/multisets are not implicitly sorted unless the source
was an ordered tree snapshot. Min/max APIs on sorted values use comparator order.

## Hash index internals (invariants for editors)

Set, Map, Multiset, Multimap and BiMap index their keys with the private
`hashIndex` (immutable.go). It no longer depends on `hash_grow`.

- **Two backings, one surface.** Natural constructors *and natural builders*
  (`NewSet`, `NewSetBuilder`, …) back the index with a builtin Go map through
  closures. Func constructors, `Freeze*` conversions and Func builders use
  `funcTable`, which stores each key's hash and probes linearly at load ≤ 1/2.
  A natural builder records `natural: newNaturalIndex[T]` so its `Build` keeps
  the map backing. If you add a constructor, route it the same way, or it
  silently falls back to the slower func table.
- **`addUnique` may overwrite on a duplicate.** It is a single map assignment
  and reports a duplicate when the map length does not change. Only call it
  from constructors that throw away the whole value on a duplicate (Map,
  BiMap). First-occurrence constructors (Set, Multiset, Multimap) must call
  `getOrAdd`/`add`, which keep the first position.
- **The index is written only during construction.** After that it is
  read-only, which is what keeps concurrent reads safe with no lock.
- **Multimap groups share one backing array.** Each group is cut as
  `flat[start:start:start+count]`, at full capacity. `Get` and `AsMap` must
  keep copying: Go slices are writable, so returning a group would let callers
  mutate the snapshot and the next key's values.
  `TestMultimapSharedGroupsAreNotExposed` pins this. Internal readers use
  `group`.
- **`ValuesOf` is the non-copying public read path.** It yields a group's
  values one at a time, so the shared slice is never handed out. Keep the
  `group` lookup *inside* the returned closure: that keeps `ValuesOf` small
  enough to inline into a range loop, which is what makes it allocation-free.
  With the lookup outside the closure it costs 3 allocations (88 B) and is
  slower than `Get`. `TestMultimapValuesOfAllocationFree` pins this.
- **The BiMap inverse shares `keys`/`values`, swapped.** Never write either
  slice after construction. `EntriesSlice` builds a fresh slice.
- **Slice iterators use a single closure literal** (`var values []T; if m != nil
  {...}; return sequence(values)`). Returning one of two closures (`if nil {
  return sequence(nil) }`) stops the iterator inlining into the range loop,
  costing 2 allocations and an indirect call per element.
  `TestHashLookupsDoNotAllocate` pins the zero-allocation lookups and
  iteration.
- Benchmarks that sum over a range-over-func iterator must use a `b.N` loop
  with a local accumulator. Inside a `b.Loop` body the compiler does not
  inline the range body.

## JSON

List/set/map/sorted/multiset/multimap/bimap/table values marshal as JSON arrays. Lists, sets, and sorted sets use
element arrays; maps, multimaps, and bimaps use lowercase `key`/`value` entries;
multisets use `value`/`count` entries; tables retain the existing cell format.
Iteration order is deterministic for each implementation.

Immutable values deliberately do not implement `UnmarshalJSON`, which would
mutate an already-published value. Decode into ordinary Go slices/entries and
call a constructor or builder instead.

## Verification and benchmarks

Tests cover mutation of constructor inputs and returned slices, builder reuse,
copy-of identity, duplicate failures, custom equality/comparison, every sorted
navigation boundary, subview retention, early stop, JSON, inverse identity,
source-to-snapshot independence, and concurrent reads under `-race`.
`index_test.go` adds fixed-seed differential tests of both index backings
against a map model (with a weak 8-value hash that forces probe chains and
growth), plus tests that natural constructors, Func constructors, builders and
a slice model all agree. It also covers non-exposure of shared groups and
inverse slices, and zero-allocation lookup/iteration. The pinned
Guava oracle covers all nine available families. Matched Go/JMH benchmarks cover
construction, copy-of, lookup/contains, count, list iteration, sorted subranges,
and multimap snapshot costs. Raw and normalized results are retained in
`docs/benchmarks/immutable/`.
