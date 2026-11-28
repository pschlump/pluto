# multimap / multimap_ts

## Shared representation

All three variants share `core[K,V,C]`, whose key index is the package's
private open-addressing `table[K, C]` (`table.go`, a copy of
`multiset/table.go` with the same invariants: power-of-two slots, Fibonacci
home slot, hash 0 remapped, backward-shift deletion, load ≤ 3/4). The slot
payload is the collection itself, so every operation hashes and probes the key
once and updates the collection in place through the slot; never go back to a
Search-then-Insert pair. `collectionOps[V,C]` supplies collection behavior
without converting keys or values to `any`:

- list C = `[]V`, with value equality for removal and membership;
- set C = `*valueSet[V]` (= `*table[V, struct{}]`);
- sorted set C = `[]V`, with binary search, insertion sorting, and deduplication.

The core tracks total associations and uses the table length for distinct keys.
The central invariant is that no table entry has an empty collection. Every
removal, removal-all, and empty replacement deletes such a key immediately.
Model tests check this after randomized operations (`TestMultimapClusteredHashModels`
also checks the table invariants with colliding key/value hashes).

PutAll first checks whether the batch could overflow the association count
(`size > maxInt - len(values)`). If it cannot, it adds in place: a new key's
collection is built completely before the key is installed; an existing key
is extended through `addAll` (list: one append — safe because stored slices
are never aliased, Get/AsMap clone; sorted: `mergeSortedBatch`, which sorts a
copy of the batch, makes every comparator call, and only then merges in place
from the back) or value by value for sets, with `size` updated per successful
add so a panicking caller callback leaves a consistent partial batch. A batch
that could overflow takes `putAllRebuild`, the original copy-and-rebuild path
that checks overflow exactly before installing. ReplaceValues still builds the
replacement first. Overflow never exposes a partial batch. List and sorted
slice removal clear the vacated final slot so references are not retained.

Representatives are observable with custom equality and are pinned by
`TestMultimapRepresentatives` (they reproduce the old hash_grow semantics): an
adding Put/PutAll/ReplaceValues stores the latest key; Remove keeps the stored
key; a set Put of a present value replaces the stored value; a set PutAll that
adds nothing changes nothing (hence the "any absent value?" pre-scan),
otherwise the last equal batch value wins; a sorted PutAll keeps existing values
and the first of equal batch values.

## Snapshot contract

Get clones values. `ValueCount` and `ValuesOf` (2026-11-27) are the
non-copying per-key readers on all three variants: `ValueCount` is
`ops.length` of the stored collection (`core.ValueCount`), and `ValuesOf`
ranges the stored collection directly. List and sorted range the `[]V`; set
walks the value table's slots in slot order, which must stay identical to
`table.keys` (what Get returns). `TestPerKeyReadersMatchGet` pins both
against Get after every random mutation, and reversing the set walk makes it
fail. Keep the lookup inside the returned closure so `ValuesOf` inlines and
stays allocation-free (`TestPerKeyReadersAllocationFree`). The twins'
`ValuesOf` returns a snapshot taken under the read lock (twin discipline).
AsMap builds a `MapSnapshot`, not a Go map: this supports
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

The linked core uses the same private table for the key index
(`table[K, *linkedKeyEntry]`) and, in the set variant, for each key's value
index (`table[V, *linkedAssociation]`). Key order is an intrusive list through
the key entries (`keyHead`/`keyTail`, `prevKey`/`nextKey`, first key
representative kept in `entry.key`); every key deletion must also call
`unlinkKey`. `put` computes every caller hash before linking anything.
PutAll looks the key up once and pre-counts additions (`addedCount`, a
temporary table) only when the batch could overflow.

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
