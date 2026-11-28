# multimap benchmarks: Pluto (Go) vs Guava (Java)

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
| ListMultimap (Java: ArrayListMultimap) | 896.06–915.75K · 4690 B, 0 allocs | 0.63M ± 1.87M |
| SetMultimap (Java: HashMultimap) | 18.37–19.68M · 26 B, 0 allocs | 36.39M ± 20.07M |
| SortedSetMultimap (Java: TreeMultimap) | 14.64–14.68M · 31 B, 0 allocs | 10.40M ± 4.55M |
| Baseline: map of lists | 904.98–906.62K · 33 B, 0 allocs | 0.62M ± 1.79M |
| Baseline: map of sets | 25.06–25.11M · 0 B, 0 allocs | 38.99M ± 25.83M |

## 2026-11-27: one-probe key tables, in-place PutAll

Rates are operations per second on the matched corpus. "Go before" is the
earlier 2026-11-27 current-results run; "Go after" is `go-2026-11-27-perf.txt`
(three 1-second runs at load average 7–10 from concurrent benchmarking, so
absolute numbers understate the change — the baselines measured in the same
session are shown for scale). The interleaved old/new A/B run under identical
load, `go-2026-11-27-perf-ab.txt` (lines prefixed `old`/`new`, old built from
the parent commit), gives SetMultimap 150.3–157.3 → 55.4–66.0 ns/op,
SortedSetMultimap 133.7–142.0 → 76.3–79.2, LinkedHashMultimap 135.9–139.7
(one 391.9 outlier) → 64.1–68.2, ListMultimap 1185–1208 → 1039–1408 (noisy;
bytes 6474 → 2631–4498), and LinkedListMultimap at parity (12.0–17.8K vs
11.3–12.3K ns/op; it is dominated by linear per-key list removal and is very
load-sensitive, so its current-results cell in phase9 is left unchanged).

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| ListMultimap (Java: ArrayListMultimap) | 886.52–894.45K · 6473 B, 0 allocs | 872.60–902.53K · 4545 B, 0 allocs | 0.63M ± 1.87M |
| SetMultimap (Java: HashMultimap) | 7.56–7.66M · 222 B, 1 alloc | 16.77–17.15M · 26 B, 0 allocs | 36.39M ± 20.07M |
| SortedSetMultimap (Java: TreeMultimap) | 8.18–8.22M · 108 B, 0 allocs | 12.59–13.29M · 31 B, 0 allocs | 10.40M ± 4.55M |
| LinkedHashMultimap (Java: LinkedHashMultimap) | 8.03–8.09M · 60 B, 0 allocs | 13.52–13.81M · 34 B, 0 allocs | 33.93M ± 2.42M |
| Baseline: map of lists (same session) | 907.44–907.44K | 889.68–920.81K | 0.62M ± 1.79M |
| Baseline: map of sets (same session) | 24.88–25.04M | 23.42–23.50M | 38.99M ± 25.83M |

Causes, from CPU and allocation profiles:

- **PutAll copied and rebuilt the whole collection** (snapshot + append +
  replace) to stay atomic on overflow. For sets that allocated a fresh
  hash_grow table per PutAll — most of the old 222 B and the 1 alloc/op. PutAll
  now checks overflow up front (a batch that cannot reach the int limit cannot
  overflow) and adds in place: list = one append, set = value-by-value insert,
  sorted = sort a copy of the batch once and merge it in place from the back
  (O(v + k log k), every comparator call before the first write). Batches that
  could overflow still take the exact copy-and-rebuild path.
- **Every write hashed and probed the key twice and re-inserted the entry**
  (hash_grow `Search`, then `Insert`, even when only the collection's contents
  changed), through a closure wrapping the caller's key functions. The key
  index is now a private open-addressing table (`multimap/table.go`, a copy of
  multiset's) whose payload is the collection, updated in place through the
  slot; SetMultimap value sets use the same table with an empty payload.
- **Linked multimaps used `linked_hash_map`** (a `map[uint64][]*node` plus a
  node allocation per key and per value-index entry) for both the key index
  and every per-key value index, looked up twice per Put/Remove, and PutAll
  built a throwaway `linked_hash_map` just to pre-count additions. Keys and
  value indexes are now the private table; key order is an intrusive list
  through the (already allocated) key entries; PutAll looks the key up once and
  only pre-counts when it could overflow. The association node per value is
  inherent to the global-order list (Guava allocates one too).

Harness mismatch (not fixed, needs an API addition): the `get` operation is
`len(Get(key))` in Go but `get(key).size()` in Java. Guava's `get` returns a
live view, so `size()` is O(1); Pluto's documented `Get` returns a fresh
snapshot slice, so 15% of operations copy every value of a key. A temporary
length-only variant of the Go harness measured SetMultimap 47.5–50.9 ns/op and
LinkedHashMultimap 53.7–56.9 ns/op against 59.5–60.2 and 63.1–72.9 with `Get`,
i.e. the snapshot is ~11 ns/op and all of the remaining allocation. This also
explains ListMultimap's KB/op (long per-key lists copied on every `get`).

Remaining SetMultimap gap to the map-of-sets baseline after the snapshot cost
is ~6–8 ns/op: two indirect function-value calls per probe (stored eq/hash)
and the extra key-entry level that Go's `map[string]map[string]struct{}`
inlines.

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 256 string keys, 16 values per key, and
10,000 operations: 45% put, 20% remove, 15% get, 10% putAll, and 10%
containsEntry. Go ran three 1-second measurements. JMH 1.37 ran two 1-second
warmups and three 1-second measurements, one fork and one thread. The run also
includes Phase 0/1a fixtures; normalized per-metric workload metadata separates
those corpora.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation                      | Runtime | Result                                    |
|-------------------------------------|---------|------------------------------------------:|
| Pluto ListMultimap                  | Go      | 809.061–817.661K ops/s; about 6.3 KiB/op  |
| Pluto SetMultimap                   | Go      | 6.242–6.631M ops/s; 222 B/op, 1 alloc/op  |
| Pluto SortedSetMultimap             | Go      | 7.189–7.331M ops/s; 108 B/op              |
| Go `map[string][]string`            | Go      | 884.956–902.527K ops/s; 32 B/op           |
| Go `map[string]map[string]struct{}` | Go      | 24.390–24.558M ops/s; 0 B/op, 0 allocs/op |
| Guava ArrayListMultimap             | JMH     | 0.630M ops/s ± 1.871M                     |
| Guava HashMultimap                  | JMH     | 36.392M ops/s ± 20.070M                   |
| Guava TreeMultimap                  | JMH     | 10.395M ops/s ± 4.551M                    |
| Java list-map baseline              | JMH     | 0.623M ops/s ± 1.787M                     |
| Java set-map baseline               | JMH     | 38.991M ops/s ± 25.826M                   |

List-heavy streams grow duplicate collections, so PutAll's snapshot/rebuild
cost is visible. SetPutAll also rebuilds the per-key set to preserve atomicity;
the Go map baseline is substantially faster on this stream. These are workload
observations, not single-ratio claims about general performance. JMH confidence
errors are large for several cases and are retained rather than hidden.

Raw files:

- `go-matched.txt`: all Go matched measurements and allocation metrics.
- `jmh-matched.json`: all JMH measurements.
- `normalized.json`: normalized metrics with environment and per-workload
  capacity, distribution, and seed.

Reproduce with `make guava-bench`.
