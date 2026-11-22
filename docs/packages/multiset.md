# multiset / multiset_ts

## Representation and invariants

`HashMultiset[T]` composes `hash_grow.HashTab[Entry[T]]`; entry equality and
hashing inspect only `Entry.Value`. The facade retains the value-level functions
for cloning, algebra adoption, and the twin. `size` is the sum of all entry
counts. The table contains exactly `DistinctLen` entries, all with Count > 0.

`TreeMultiset[T]` is a dedicated AVL tree rather than a `tree_set` facade. Each
node stores one value, its positive count, height, and children. The tree keeps
`distinct` and total `size` separately. This is necessary for O(log n + k)
positioned range queries: `tree_set.Range` currently performs a filtered O(n)
walk and has no lower-bound iterator to reuse.

After every randomized mutation, tests check ordering, positive counts, cached
heights, and the AVL height difference. Range snapshots are built from sorted
entries by midpoint recursion, producing a balanced independent tree in O(k)
after the O(log n + k) range walk.

## Mutation contracts

Counts and total sizes are signed `int`. Check before changing either. Add and
SetCount check both per-value and total overflow. AddSlice and Sum build a fresh
value and install it only after success, so overflow does not expose a partial
batch. Remove saturates at zero. Zero counts never create entries. Constructors
are required only when an operation actually stores a positive count.

`SetCount` deliberately returns the old count (Guava convention), while Add and
Remove return the resulting count. Differential traces query Count after every
mutation so their result stream does not depend on that difference.

## Algebra and aliases

Union=max, Sum=addition, Intersection=min, Minus=saturating subtraction. These
are destination-form operations like Pluto's set algebra, not mutating merges.
Snapshot at least the first operand before rebuilding and install only at the
end; both operands may alias the destination. A zero destination adopts yy's
functions, then zz's. Function consistency is a documented precondition.

## Twin lock discipline

`multiset_ts.HashMultiset` contains a plain value plus `sync.RWMutex`. Single
and compound mutations take the write lock; AddSlice and Sum are therefore
atomic to readers. Iterator constructors clone under a read lock and return a
slice-backed sequence, so callers may mutate the set during iteration and early
stop never retains a lock.

Equal, ContainsOccurrences, Union, Sum, Intersection, and Minus snapshot each
operand independently. No method holds two multiset locks. Opposite-direction
calls cannot deadlock. Algebra is atomic on the destination only, not a
cross-set transaction. JSON decoding and encoding happen on snapshots or before
locking so user-defined JSON methods do not run under the twin lock.

## Tests and oracles

`multiset/testdata/multiset.trace.json` and `.golden.json` use the Phase 0 Guava
`HashMultiset` oracle at the pinned commit. Normal tests only read the files.
The same trace runs against HashMultiset and TreeMultiset and distinguishes
Len from DistinctLen. Fixed-seed models use `map[int]int`; tests also cover
zero removal, occurrence iteration, overflow, aliases, custom functions, JSON,
open/closed bounds, range independence, AVL invariants, twin atomicity,
snapshots, opposite-direction operations, and `-race`.

## Benchmarks

`make guava-bench` runs the common Phase 0 corpus through Guava HashMultiset,
Guava TreeMultiset, Java HashMap, Pluto HashMultiset, Pluto TreeMultiset, and a
Go map. `multiset/bench_test.go` additionally covers hit/miss Count, distinct
and occurrence iteration, Sum/Intersection at duplicate ratios 1/4/16, and
GC-stable retained-memory differences for 4096 distinct values at those ratios.
Memory measurements are Go-only; do not compare them numerically with a Java
heap measurement made by a different method.
