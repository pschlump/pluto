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
equality callbacks run under the lock and must not re-enter the same interner.
No callback runs during `Len`. The package is safe for concurrent use and has no
separate `_ts` twin.

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
