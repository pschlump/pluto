# immutable benchmarks: Pluto (Go) vs Guava (Java)

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
| List build | 458.51–482.86K · 32792 B, 2 allocs | 1.39M ± 0.06M |
| List copy-of | 579.04–582.75M · 0 B, 0 allocs | 452.74M ± 282.14M |
| List `At` | 1.90–1.92G · 0 B, 0 allocs | 484.09M |
| List full iteration | 972.76–980.39K · 0 B, 0 allocs | 725K ± 19K |
| Set build | 15.46–15.61K · 180776 B, 26 allocs | 34.15K ± 13.80K |
| Set contains | 189.79–193.09M · 0 B, 0 allocs | 307.83M ± 25.43M |
| Map build | 16.00–16.14K · 316912 B, 24 allocs | 20.03K ± 1.87K |
| Map get | 122.59–123.81M · 0 B, 0 allocs | 168.65M ± 8.27M |
| Sorted set build | 3.29–3.38K · 32816 B, 3 allocs | 4.15K ± 0.22K |
| Sorted set contains | 22.96–24.45M · 0 B, 0 allocs | 35.48M ± 3.77M |
| Sorted set subrange | 22.60–22.86M · 32 B, 1 alloc | 20.15M ± 1.97M |
| Sorted map build | 27.86–28.20K · 98352 B, 3 allocs | 18.12K ± 0.57K |
| Sorted map get | 16.28–16.56M · 0 B, 0 allocs | 19.97M ± 3.66M |
| Multiset build | 12.36–12.41K · 325417 B, 40 allocs | 9.91K ± 1.88K |
| Multiset count | 160.75–165.43M · 0 B, 0 allocs | 234.25M ± 16.02M |
| Multimap build | 26.05–26.18K · 77648 B, 33 allocs | 31.55K ± 4.81K |
| Multimap get | 15.85–16.29M · 512 B, 1 alloc | 277.37M ± 17.80M |
| Multimap count of one key's values (`ValueCount`) | 152.18–166.56M · 0 B, 0 allocs | 277.37M ± 17.80M |
| Multimap values of one key (`ValuesOf`, no copy) † | 43.01–43.71M · 0 B, 0 allocs | — |
| BiMap build | 8.70–8.73K · 464962 B, 48 allocs | 12.26K ± 1.04K |
| BiMap get | 126.34–129.28M · 0 B, 0 allocs | 174.61M ± 8.27M |

Retained memory per family (Go bytes per input vs Guava JOL bytes per input) is in the original record below.

`ValueCount` (added 2026-11-27) is the same work as Guava's benchmark, `get(key).size()`, so it has a real Java comparison. `Get` must copy, because it returns a slice the caller owns.

† `ValuesOf` (added 2026-11-27) counts a key's values by iterating them, without the copy `Get` must make. Guava's benchmark only takes the size of a live view, which is O(1), so the two don't do equal work and there is no Java figure for this row.

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 4096 unique string keys, and 4096 seeded
uniform integer values. Multimaps use 64 keys. Construction benchmarks process
all 4096 inputs; iteration covers a full list; sorted subranges select the middle
half of the value universe. Go ran three 1-second measurements. JMH 1.37 ran two
1-second warmups and three 1-second measurements, one fork and one thread.
Allocations remain separate from the normalized comparison rates.

## Construction, copy, and lookup

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Family/workload     | Pluto Go                                  | Guava JMH                 |
|---------------------|------------------------------------------:|--------------------------:|
| List build          | 460.405–475.059K ops/s; 32792 B, 2 allocs | 1.391M ops/s ± 0.056M     |
| List copy-of        | 559.597–563.063M ops/s; 0 B, 0 allocs     | 452.735M ops/s ± 282.138M |
| List At             | 1.828–1.866G ops/s; 0 B, 0 allocs         | 484.086M ops/s            |
| List full iteration | 202.922–204.499K ops/s; 24 B, 2 allocs    | 0.725M ops/s ± 0.019M     |
| Set build           | 5.288–5.339K ops/s; 480361 B, 42 allocs   | 34.151K ops/s ± 13.799K   |
| Set contains        | 101.348–103.220M ops/s; 0 B, 0 allocs     | 307.828M ops/s ± 25.433M  |
| Map build           | 4.632–4.688K ops/s; 627169 B, 28 allocs   | 20.034K ops/s ± 1.865K    |
| Map get             | 53.476–55.218M ops/s; 0 B, 0 allocs       | 168.651M ops/s ± 8.271M   |
| Sorted set build    | 3.240–3.317K ops/s; 32816 B, 3 allocs     | 4.154K ops/s ± 0.221K     |
| Sorted set contains | 24.704–25.674M ops/s; 0 B, 0 allocs       | 35.480M ops/s ± 3.774M    |
| Sorted set subrange | 21.473–21.617M ops/s; 32 B, 1 alloc       | 20.153M ops/s ± 1.971M    |
| Sorted map build    | 27.488–27.917K ops/s; 98352 B, 3 allocs   | 18.119K ops/s ± 0.572K    |
| Sorted map get      | 15.218–15.404M ops/s; 0 B, 0 allocs       | 19.967M ops/s ± 3.661M    |
| Multiset build      | 5.053–5.123K ops/s; 570481 B, 42 allocs   | 9.911K ops/s ± 1.884K     |
| Multiset count      | 91.158–91.743M ops/s; 0 B, 0 allocs       | 234.254M ops/s ± 16.024M  |
| Multimap build      | 13.127–13.303K ops/s; 79112 B, 478 allocs | 31.545K ops/s ± 4.810K    |
| Multimap get        | 13.970–14.168M ops/s; 512 B, 1 alloc      | 277.371M ops/s ± 17.801M  |
| BiMap build         | 2.247–2.359K ops/s; 1118560 B, 56 allocs  | 12.257K ops/s ± 1.037K    |
| BiMap get           | 54.377–55.804M ops/s; 0 B, 0 allocs       | 174.608M ops/s ± 8.272M   |

Pluto Multimap `Get` returns an allocated snapshot, while Guava returns a live
view; this is intentionally an API-cost comparison. Copy-of returns the existing
immutable value on both sides. Do not infer a single cross-runtime speed ratio
from latency and throughput measured by different frameworks.

## Retained memory

Go uses GC-stable heap-allocation differences with corpus inputs retained before
the baseline. Java uses JOL `GraphLayout.totalSize`, which includes reachable
shared input objects. The methods and resulting numbers are not directly
comparable. Detailed results are in `go-memory.txt` and `java-memory.json`.

| Family     | Go retained bytes (per input) | Guava JOL bytes (per input) |
|------------|------------------------------:|----------------------------:|
| List       | 32768 (8.00)                  | 81104 (19.80)               |
| Set        | 224128 (54.72)                | 67968 (16.59)               |
| Map        | 360672 (88.05)                | 392424 (95.81)              |
| Sorted set | 32816 (8.01)                  | 51608 (12.60)               |
| Sorted map | 98352 (24.01)                 | 294208 (71.83)              |
| Multiset   | 254208 (62.06)                | 129792 (31.69)              |
| Multimap   | 40256 (9.83)                  | 88328 (21.56)               |
| BiMap      | 655840 (160.1)                | 437232 (106.7)              |

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.
- `go-memory.txt` and `java-memory.json`: retained-memory measurements.

Reproduce matched measurements and JOL output with `make guava-bench`. Run Go
memory separately:

```sh
go test ./immutable -run '^$' -bench 'BenchmarkImmutableMemory$' \
  -benchmem -benchtime=100ms -count=1
```

## 2026-11-27: Purpose-built hash index, flat multimap groups, inlinable iterators

Interleaved A/B on the same Apple M4 Max (darwin/arm64, Go 1.27.0): three
rounds, each running the previous version (31a1d71, built in a temporary
worktree) and then this version, 1-second runs. Other agents were
benchmarking on the machine at the same time (load average 5–8 on 14
cores), so ranges are wider than on a quiet machine; the sorted rows,
whose lookup code did not change, moved by up to ±10% between binaries
and give a feel for the noise. Raw output: `go-2026-11-27-hash-index.txt`.
Guava figures are the 2026-11-22 JMH record, not re-run.

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| List full iteration | 198.10–206.65K · 24 B, 2 allocs | 938.97–969.93K · 0 B, 0 allocs | 725K ± 19K |
| Set build | 5.92–6.03K · 480361 B, 42 allocs | 15.22–15.63K · 180776 B, 26 allocs | 34.15K ± 13.80K |
| Set contains | 104.54–111.86M · 0 B, 0 allocs | 185.67–191.72M · 0 B, 0 allocs | 307.83M ± 25.43M |
| Map build | 5.16–5.46K · 627172 B, 28 allocs | 13.43–16.23K · 316913 B, 24 allocs | 20.03K ± 1.87K |
| Map get | 66.80–67.57M · 0 B, 0 allocs | 115.14–118.06M · 0 B, 0 allocs | 168.65M ± 8.27M |
| Multiset build | 5.64–5.84K · 570482 B, 42 allocs | 10.28–12.45K · 325417 B, 40 allocs | 9.91K ± 1.88K |
| Multiset count | 99.50–101.73M · 0 B, 0 allocs | 158.50–163.99M · 0 B, 0 allocs | 234.25M ± 16.02M |
| Multimap build | 15.01–15.68K · 79112 B, 478 allocs | 22.18–26.12K · 77648 B, 33 allocs | 31.55K ± 4.81K |
| Multimap get | 15.10–16.13M · 512 B, 1 alloc | 12.54–16.61M · 512 B, 1 alloc | 277.37M ± 17.80M |
| BiMap build | 2.50–2.68K · 1118561 B, 56 allocs | 7.06–8.66K · 464962 B, 48 allocs | 12.26K ± 1.04K |
| BiMap get | 65.19–67.43M · 0 B, 0 allocs | 121.65–123.76M · 0 B, 0 allocs | 174.61M ± 8.27M |

Retained memory (`BenchmarkImmutableMemory`) also fell for every hash
family: Set 224128 → 180752 B, Map 360672 → 316912 B, Multiset 254208 →
205368 B, Multimap 40256 → 39504 B, BiMap 655840 → 464960 B.

### Causes (profile evidence)

- **Lookups went through four indirect calls into a general-purpose table.**
  Every hash family was indexed by a `hash_grow.HashTab[indexedValue[K]]`
  whose equality and hash funcs were closures wrapping the natural-constructor
  closures. In the Map get CPU profile, `hash_grow.Search` was 42% flat,
  `maphash.comparableHash[string]` 29% cumulative and the four wrapper
  closures most of the rest; the actual string hash (`memHashAES`) was 13%.
  A plain Go `map[string]int32` looked up through one closure measured 6.5
  ns against 15.4 ns (and `map[int]` 4.0 ns against 9.6 ns for Set contains).
- **Builds grew the index from 16 slots and hashed every key twice.** The
  table was created with capacity 16 and doubled about nine times for 4096
  keys (most of the 42 allocations and 480–627 KB per build), and every
  constructor did a `get` followed by an `add` that searched again.
- **Multimap build appended each value to its own per-key slice**, so 64
  keys × ~7 growth steps produced 478 allocations.
- **BiMap build copied every entry twice**: once for the forward map and once,
  swapped, for the inverse.
- **List iteration allocated and could not inline.** `All` returned one of
  two closure literals (`sequence(nil)` or `sequence(values)`), so the
  iterator could not be inlined into the range loop: the range body escaped
  (2 allocations) and every element paid an indirect `yield` call.

### What changed

- `hashIndex` is now private to `immutable`, with two backings behind the same
  three closures (`lookup`, `getOrAdd`, `addUnique`). Natural constructors and
  natural builders (`NewSet`, `NewMap`, `NewMultiset`, `NewMultimap`,
  `NewBiMap`, `New*Builder`) use a builtin Go map presized from the input
  length. Func constructors use a purpose-built linear-probing table that
  stores each key's hash (probes compare hashes before calling the caller's
  equality function; growth reuses stored hashes), is presized from the input
  length and holds load ≤ 1/2. No data passes through an interface.
- `getOrAdd` gives one index operation per input element for Set, Multiset and
  Multimap. `addUnique` lets Map and BiMap insert with a single map assignment
  (a duplicate shows up as an unchanged map length). Those constructors throw
  away the whole value on a duplicate, so overwriting the stored position on
  that path does no harm.
- Multimap builds in two passes: positions and per-key counts first, then one
  shared backing array cut into full-capacity per-key groups. `Get` and
  `AsMap` still return fresh copies, so the shared array is never exposed;
  `ContainsKey`, `ContainsEntry` and `Equal` now read the group in place
  instead of copying it.
- BiMap stores parallel `keys`/`values` slices. The inverse shares both,
  swapped, instead of holding its own copy, and duplicate checks run before
  anything is copied.
- Every `All`/`Entries`/`Keys`/`Values`/`Reverse` that returns a slice
  sequence builds one closure literal over a possibly-nil slice, so the
  iterator inlines into the caller's range loop (0 allocations).
- `Multiset.Count` moved its tree path out of line (`treeCount`).

### Harness fix

`BenchmarkImmutableListIteration` used `for b.Loop()` and added every element
straight into a package global. Inside a `b.Loop` body the compiler keeps
every assignment alive and does not inline the range-over-func body, so each
element cost an indirect call plus a memory round trip. Measured in one
binary: 4.9 µs per pass with `b.Loop` and the global, 7.6 µs with `b.Loop` and
a local, 1.03 µs with a `b.N` loop and a local. JMH's method sums into a local
and returns it, so the Go benchmark now does the same (a `b.N` loop, one
store to the global per pass). The harness fix alone does not explain the
gain: the old implementation under the new harness still runs at
203.58–207.47K with 2 allocations, because its iterator cannot inline.

### Not changed, and why

- **Multimap get** is unchanged. The documented contract is that `Get` returns
  an independent slice the caller may mutate, and a Go slice cannot be handed
  out read-only, so the 512-byte copy and its allocation stay. The profile is
  dominated by GC work caused by that allocation. Guava returns a live,
  immutable view. Recommended API addition (not made): a non-copying
  accessor such as `ValuesOf(key) iter.Seq[V]` plus `ValueCount(key) int`.
  Those would match Guava's `get(key).size()` workload with zero allocations.
- **List build** stays at about 2 µs. NewList itself is about 3% of CPU; the
  rest is allocating and filling a fresh 32 KB array, plus GC on the
  benchmark's tiny live heap. Java copies 16 KB of compressed references into
  a TLAB.

### Rejected experiments

- Calling the index closure directly from `Map.Get`, skipping the inlined
  `get` wrapper, so `Get` itself might inline: it still did not inline (cost
  91 > 80) and measured 8.3–8.4 ns against 8.1–8.2 ns. Reverted.
- Using the func-mode open-addressing table for the natural constructors too:
  10.3 ns per string lookup against 6.5 ns for the builtin map. The builtin
  map is used.
- Trimming Set's presized value slice when duplicates leave it under 75%
  full: it adds a copy per build, and the old append-doubling slice had as
  much slack anyway. Dropped without a separate measurement.
