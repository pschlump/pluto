# intern

`Interner[T]` provides strong, concurrent canonicalization. `Intern(value)`
returns the first stored value equal to `value`; later equal values do not
replace it. Values remain reachable until `Clear` or until the interner itself
becomes unreachable. There is no weak-lifetime or finalizer behavior.

`New` uses `==` and `maphash` for comparable values. `NewFunc` accepts matching
equality and hash functions for arbitrary values. Equal values must have equal
hashes. The zero value is readable as empty but must be constructed before use.

| Operation | Contract | Complexity |
|---|---|---|
| Intern | Return the canonical first equal value | expected O(1) |
| Len / Contains | Number/membership of canonical values | O(1) / expected O(1) |
| Clear | Release all canonical values | O(n) |

A single RWMutex makes each operation atomic. Hash callbacks run before locking;
equality callbacks run under the lock (read lock for lookups, so they may run
concurrently with each other) and must not re-enter the same interner. No
callback runs during `Len`. The package is safe for concurrent use and has no
separate `_ts` twin.

## Internals (2026-11-27)

`Intern` looks up under the **read** lock; only a miss takes the write lock,
and it must **re-check** before inserting (another goroutine may have inserted
the same class in between — dropping the re-check breaks the first-value
identity guarantee under concurrency).

- **`New`: native index.** `native` holds closures over a built-in `map[T]T`
  (Interner's `T any` cannot declare `map[T]` itself). Values pass by value
  through function values — no interface boxing. The stored maphash `hash` is
  kept for the constructed-or-zero test (`hash != nil`) but the native path
  never calls it. The hit path is inlined in `Intern` (lock, `get`, unlock);
  the miss path is the separate `internNativeMiss`, so the hit path has no
  `defer`.
- **`NewFunc`: linear-probing table** of `slot{hash, value}` cells (values
  stored inline, no per-value allocation), power-of-two sized, lazily
  allocated, doubled when `(length+1)*4 > len(slots)*3`. Home cell
  `(hash*fibMul) >> shift`. The caller hash is cached via `cellHash`, which maps
  0 to 1 because **hash 0 marks an empty cell** — equal values still get equal
  cached hashes. No deletion exists (only `Clear`), so no tombstones or shifts.

`TestTableDifferentialModel` (identity, mod-5, all-zero and 0/1 hashes, tagged
values equal by key so the first-stored tag proves canonical identity) and
`TestNativeDifferentialModel` (backing-array identity via `unsafe.StringData`)
are the oracles; `TestHitAllocations` pins zero allocations for a hit and for
`Contains`. Removing the 0→1 remap or returning the probe instead of the
canonical value both fail them.

The old layout was `map[uint64][]entry` for every interner: maphash, a second
runtime hash of the uint64, a bucket-slice walk through the `equal` closure,
always under the write lock, plus one slice allocation per distinct value
(10,084 allocations to build 10,000 values; now 88).

## Verification and benchmarks

`intern/testdata/guava_intern.json` is generated from Guava's strong `Interner`
by `Phase9Oracle`. Tests cover canonical identity, collisions, Clear, zero-value
reads, strings, and concurrent canonicalization under `-race`.

`make guava-bench` measures mixed hit/miss canonicalization against Guava and a
Go map+RWMutex baseline. `BenchmarkInternerBuild` reports construction
allocations and `BenchmarkInternerRetained` reports GC-stable heap growth for
10,000 retained strings. `Phase9Memory` measures Guava with JOL GraphLayout.
Go heap growth and JOL retained size use different methods and are not directly
comparable. Strong retention is intentional; this is not a weak-cache benchmark.
