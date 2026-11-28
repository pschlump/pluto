# phase9 benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs on a quiet machine after the 2026-11-27 rewrites, shown
slowest–fastest, plus bytes and allocations per operation. Raw output is
`go-2026-11-27-current.txt`; the dated sections below give the causes and
before/after comparisons.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| Strong intern, hit/miss | 49.70–50.08M · 16 B, 1 alloc | 40.34M ± 4.19M |
| Baseline: map + RWMutex interner | 55.25–55.56M · 16 B, 1 alloc | — |
| Linked map put/get (Java: LinkedHashMap) | 45.52–45.83M · 0 B, 0 allocs | 132.53M ± 34.46M |
| Linked multiset operation | 30.18–31.79M · 0 B, 0 allocs | 64.82M ± 31.19M |
| Linked list multimap operation | 142.31–143.20K · 2646 B, 0 allocs | 69.17K ± 246.28K |
| Linked hash multimap operation | 16.44–17.02M · 34 B, 0 allocs | 33.93M ± 2.42M |

## Original record (2026-11-22)

Recorded 2026-11-23 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
JMH 1.37, and Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`.
Go ran three 1-second measurements. JMH ran two 1-second warmups and three
1-second measurements, one fork/thread. Linked collections use the same seeded
multiset/multimap corpora as Phases 1a/1b. Interning uses 10,000 strings with
50% existing and 50% new canonical values.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Workload                       | Pluto/Go                                | Guava/Java or baseline                        |
|--------------------------------|----------------------------------------:|----------------------------------------------:|
| Strong intern hit/miss         | 35.842–35.984M ops/s; 16 B, 1 alloc     | Guava: 40,341,748 ops/s ± 4,192,691           |
| Map+RWMutex baseline           | 53.447–53.937M ops/s; 16 B, 1 alloc     | Go baseline, not Java                         |
| Linked map put/get             | 24.010–24.184M ops/s; 0 B, 0 allocs     | LinkedHashMap: 132,527,419 ops/s ± 34,457,924 |
| Linked multiset operation      | 15.640–15.718M ops/s; 4 B, 0 allocs     | Guava: 64,822,847 ops/s ± 31,189,604          |
| Linked list multimap operation | 58.551–59.154K ops/s; 2,646 B, 0 allocs | Guava: 69,169 ops/s ± 246,284                 |
| Linked hash multimap operation | 7.570–7.680M ops/s; 60 B, 0 allocs      | Guava: 33,931,813 ops/s ± 2,424,405           |

Zero allocs/op is the rounded steady-state average; allocated bytes come from
occasional growth. The persistent list workload accumulates duplicates and long
per-key lists, so its linear list removal cost dominates. Do not treat it as a
bounded-cache or uniform-size workload. No cross-language speed ratio is inferred.

## Strong-interner memory

Building 10,000 Go entries allocated 1,468,432–1,468,469 bytes in 10,084
allocations. GC-stable heap growth was 81.62 retained B/value. Guava's JOL
`GraphLayout.totalSize` was 866,136 bytes (86.61 B/value).

These methods are deliberately not compared numerically: Go measures heap growth
against a baseline where the input strings already exist, while JOL walks the
interner and the reachable Java strings. Weak interning and collection under GC
pressure are outside this measurement.

Raw files are `go-matched.txt`, `jmh-matched.json`, `normalized.json`,
`go-memory.txt`, and `guava-memory.json`. The complete `make guava-bench` run
also matched all Phase 9 golden files and completed normalization and JOL steps.

## 2026-11-27: linked_hash_map and intern index rewrite

Apple M4 Max, darwin/arm64, Go 1.27.0, shared with other benchmark jobs (load
average 3.6–7.1 during the runs). "Before" is the previous commit's code copied
from git into a scratch module and run with the same benchmark files; before
and after alternated in two rounds of three 1-second runs each. Raw output:
`go-2026-11-27-linked-intern-interleaved.txt` (both sides, with load per
round) and `go-2026-11-27-linked-intern-baseline.txt` (the untouched code and
harness, run first). Java numbers are the 2026-11-22 JMH record.

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| Strong intern, hit/miss | 33.68–34.88M · 16 B, 1 alloc | 46.27–49.14M · 16 B, 1 alloc | 40.34M ± 4.19M |
| Baseline: map + RWMutex interner (unchanged code) | 48.26–54.05M · 16 B, 1 alloc | 45.33–54.79M · 16 B, 1 alloc | — |
| Linked map put/get (Java: LinkedHashMap) | 22.06–23.27M · 0 B, 0 allocs | 42.52–44.94M · 0 B, 0 allocs | 132.53M ± 34.46M |
| `BenchmarkMapPutGet` (int keys, package bench) | 26.98–30.12M | 45.68–47.01M | — |
| `BenchmarkBuiltinMapPutGet` (Go map, same work) | 79.62–82.58M | 80.06–83.40M | — |
| `BenchmarkMapIteration` (10,000 entries per op) | 34.9–44.2K | 41.0–57.0K | — |
| `BenchmarkInternerBuild` (10,000 values) | 1,468,442 B, 10,084 allocs | 1,308,562 B, 88 allocs | — |
| `BenchmarkInternerRetained` | 81.62 B/value | 65.63 B/value | (JOL 86.61 B/value, different method) |

**Causes.** Both packages indexed every instance — including the natural
`New` constructors — as `map[uint64][]*node` / `map[uint64][]entry`: each lookup
ran maphash on the key, then the runtime hashed that uint64 again for the Go
map, then walked a separately allocated bucket slice (an extra dependent load)
and called the `equal` closure. The linked-map workload does `Put`+`Get`, so it
paid that twice per operation; the profile had `find` at 80% of CPU, a third of
it in `mapaccess*_fast64`. The interner additionally took the **write** lock on
every call (hits included) and allocated one bucket slice per distinct value.

**Changes.**
- `New` (comparable keys/values) now indexes through a Go built-in
  `map[K]*node` (`map[T]T` for intern) reached through closures created in a
  `comparable`-constrained constructor — function values, no interface boxing,
  one hash per lookup. `Clone`/`UnmarshalJSON` (plain and twin) keep the kind.
- `NewFunc` now uses an open-addressing linear-probing table that caches the
  caller hash per cell (one `hash` call per operation, hash-compare before
  `equal`, no rehash on growth), load ≤ 3/4, Fibonacci home-cell mixing;
  linked_hash_map deletes by backward shift. This is the path
  `multimap`'s linked variants use (they construct via `NewFunc`).
- intern: read-locked lookup, write lock + re-check only on a miss; the hit
  path is inlined in `Intern` without `defer`.
- linked_hash_map `All`/`Keys`/`Values`/`EntriesSlice`/`Clone` walk the node
  list directly instead of nesting through the `Entries` iterator.
- `linked_hash_map_ts.UnmarshalJSON` delegates to the plain method for a
  constructed inner map so a decoded twin keeps the native index; the decode
  now runs entirely under the write lock (it already called user functions
  under it). Nil/zero-value inner behavior is unchanged.

**Harness fixes.** `BenchmarkLinkedHashMap` stored every `Get` result into a
package global (JMH returns it to a Blackhole); it now accumulates into a local
and publishes once — measured effect within noise (27.1 → 28.2 ns/op on the
same code). `BenchmarkMapIteration` discarded its sum (`_ = sum`); it now
publishes it and reports allocations. Its noise (±20% run to run) persisted
with fixed iteration counts (`-benchtime=100000x`) and `-cpu=1` while the
compute-bound benchmarks in the same runs were stable to ±2%: it is a
dependent pointer chase over ~480 KB of nodes, so it tracks shared-cache
contention from concurrent jobs. Not a harness artifact we can remove.

**Workload notes.** Both sides allocate once per intern operation for the key
copy (`strings.Clone` in Go, `new String` in Java — Java's copy shares the
backing array); it is not the interner. After the first pass over the 10,000
values every intern call is a hit, on both sides. The remaining linked-map gap
to Java is structural: a Go built-in `map[string]int` doing exactly the same
put+get runs ~20 ns/op on this machine (prototype, not committed), so the
linked map is now within ~12% of Go's own map. Java's `String.hashCode` is
cached in the object and `equals` short-circuits on the identical references
the benchmark passes; Go must hash the 11-byte string on every lookup.

**Rejected.** (1) Custom open-addressing table for `New` too (one code path):
27–28 ns/op vs 22 ns/op for the built-in-map closures — the built-in map's
inlined string hash/compare beats maphash plus an `equal` closure call; kept
for `NewFunc` only. (2) Splitting linked_hash_map `find` into a native fast
path plus out-of-line table path to get it inlined into `Get`: the closure
call kept it over the inline budget (cost 137 > 80); no measurable change,
reverted.
