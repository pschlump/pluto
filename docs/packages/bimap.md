# bimap / bimap_ts

## Two indexes and coupled inverses

A `direction[K,V]` holds forward `HashTab[Association[K,V]]`, reverse
`HashTab[Association[V,K]]`, and equality/hash functions. Construction creates
both tables and two direction objects with their table pointers exchanged. The
forward and inverse HashBiMap objects point at each other permanently, giving
inverse-of-inverse identity without lazy initialization or allocation on later
Inverse calls.

Put checks both directions before mutation. A value owned by another key returns
ErrValueConflict with no changes. Updating a key deletes its old reverse entry.
ForcePut additionally deletes the key that owned the desired value. DeleteKey
and DeleteValue remove both indexes. These permutations and a fixed-seed
two-map model are tested directly.

Clone creates a new pair of tables and a new inverse cycle. Equal uses forward
entries plus value equality and assumes consistent functions. Iteration is live
and unordered in the plain package.

## JSON without detaching views

Unmarshal builds and validates a separate bimap first. Only after success does
it Clear and refill the existing tables in place. It must not replace table
pointers: previously returned inverse objects hold those pointers and would be
detached. The same rule applies to the twin. Malformed input and conflicts leave
the original unchanged.

## Twin lock discipline

The twin has forward and inverse wrapper objects over the plain inverse pair,
plus one shared lock object. Every read or mutation uses that lock regardless of
direction. Entries and EntriesSlice snapshot under the lock. Equal and Clone use
independent plain snapshots, never two twin locks at once. JSON encoding uses a
snapshot and decoding happens before locking.

Concurrent tests mutate through both directions, verify inverse identity and
deletion, exercise opposite-direction equality, and run under `-race`.

## Oracle and benchmarks

`bimap/testdata/bimap.trace.json` is executed by pinned Guava HashBiMap. Put
conflicts are canonicalized as false, missing lookups/deletions as null, and
final entries sorted by key. Ordinary tests read only the committed golden.

The bimap corpus uses seed 1729 and 4096 keys/values: 20% put, 10% forcePut,
10% deleteKey, 10% deleteValue, 25% getValue, and 25% getKey. Matched benchmarks
compare Guava HashBiMap, Pluto HashBiMap, and manually synchronized Go/Java maps.
A separate 4096-entry inverse-iteration benchmark and Go GC-stable retained
memory benchmark cover view traversal and two-index overhead. Raw and normalized
results live under `docs/benchmarks/bimap/`; Go memory numbers are not directly
comparable with Java heap measurements.
