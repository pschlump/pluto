# multimap / multimap_ts

## Shared representation

All three variants share `core[K,V,C]`, a `hash_grow.HashTab` of key entries
whose collection has type C. `collectionOps[V,C]` supplies collection behavior
without converting keys or values to `any`:

- list C = `[]V`, with value equality for removal and membership;
- set C = `*hash_grow.HashTab[V]`;
- sorted set C = `[]V`, with binary search, insertion sorting, and deduplication.

The core tracks total associations and uses the table length for distinct keys.
The central invariant is that no table entry has an empty collection. Every
removal, removal-all, and empty replacement deletes such a key immediately.
Model tests check this after randomized operations.

PutAll combines a copy of existing and new values, then builds the replacement
collection before checking size overflow and installing it. ReplaceValues does
the same for a complete replacement. Failed batches do not partially mutate the
map. List and sorted slice removal clear the vacated final slot so references
are not retained.

## Snapshot contract

Get clones values. AsMap builds a `MapSnapshot`, not a Go map: this supports
non-comparable custom key types and avoids falsely promising write-through
behavior. MapSnapshot.Get and Entries clone value slices as well. EntriesSlice
is the snapshot boundary used by the twin. Plain iterators should not be used
while mutating the same map; twin iterators are safe snapshots.

Keys repeats once per association, while KeySet is distinct. KeyMultiset builds
a `multiset.HashMultiset` with the same key equality/hash functions. Immutable
multimap conversion is deferred until Phase 2.

## Utilities

Index preserves input order per key. ForMap has unspecified key order because
Go map iteration is unspecified. InvertFrom materializes source entries before
writing the destination, making alias use safe and preventing nested locks in
the twin. The current utility constructors use natural comparable equality;
the concrete types retain custom-function constructors.

## Twin discipline

Each twin contains one plain value and one RWMutex. Compound mutations hold the
write lock. Iterators materialize under a read lock and release before caller
code. No method holds locks belonging to two multimaps. Opposite-direction
inversion and concurrent reader/writer tests run under `-race`.

## Tests and oracles

`multimap/testdata/multimap.trace.json` is executed by the pinned Guava
`MultimapOracle` against ArrayListMultimap, HashMultimap, and TreeMultimap.
Canonical results sort unordered set values but preserve list order and
duplicates. The committed `.golden.json` is read by ordinary Go tests; Java is
never a default test dependency.

Fixed-seed models cover list order and set membership, replacement returns,
flattened counts, empty-key cleanup, custom equality/hash/comparison, snapshot
independence, inversion, key multiplicity, and utilities. Twin tests cover
parity, batch atomicity, concurrent readers/writers, snapshot iteration, and
opposite-direction inversion.

## Benchmarks

The Phase 0 harness generates `multimap.corpus.json` with seed 1729, 256 keys,
16 values per key, and 10,000 operations: 45% put, 20% remove, 15% get, 10%
putAll, and 10% containsEntry. `make guava-bench` matches Guava
ArrayListMultimap/HashMultimap/TreeMultimap, Pluto's three variants, and
Go/Java list-map and set-map baselines. Raw and normalized results are retained
under `docs/benchmarks/multimap/`.

## Insertion-ordered variants and live views

`LinkedListMultimap` and `LinkedHashMultimap` preserve first-key insertion order.
Their flattened `Entries`, `Keys`, and `Values` follow Guava's global association
insertion order, not key-grouped order. `Get` retains per-key list/value order.
Removing an association and adding it again appends it globally; replacing or
removing a key removes its old global entries. The set variant deduplicates
values. Each association node has global and per-key links. Set lookup/removal
is expected O(1); list removal is O(values-per-key), and removal-all is O(v).
Key and global iteration are O(N).

`GetView(key)` returns a live `ValuesView` with Add/Put, AddAll, Remove, Replace,
Clear, membership, length, and snapshot iteration. `AsMapView` returns a live
`MapView` whose `Get` produces coupled value views and whose mutations delegate
to the parent. Existing `Get` and `AsMap` snapshot APIs are unchanged. Views work
with every plain and `_ts` variant. Each `_ts` view operation takes the parent
lock independently; compound sequences are not transactions and no lock is held
during iteration.

`Phase9Oracle` checks initial/final global and key order against Guava
`LinkedListMultimap` and `LinkedHashMultimap`. Seeded models cover duplicates,
PutAll, removal, removal-all, and reinsertion. View tests verify both mutation
directions, empty-key cleanup, unique/sorted behavior, ordering, and concurrent
snapshot access.
