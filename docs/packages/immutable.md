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

Immutable RangeSet and RangeMap remain deferred until their mutable bases land in
Phase 3.

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

## JSON

All new immutable values marshal as JSON arrays. Lists, sets, and sorted sets use
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
source-to-snapshot independence, and concurrent reads under `-race`. The pinned
Guava oracle covers all nine available families. Matched Go/JMH benchmarks cover
construction, copy-of, lookup/contains, count, list iteration, sorted subranges,
and multimap snapshot costs. Raw and normalized results are retained in
`docs/benchmarks/immutable/`.
