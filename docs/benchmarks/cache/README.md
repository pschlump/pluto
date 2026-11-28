# Phase 4 cache benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 4096 keys, and 10,000 operations (80% get,
20% put). Separate keys provide Zipf, scan, and phase-change workloads. Eviction
capacity is 256. Expiration/refresh advances a fake clock by 2 ns against a 1 ns
policy. Go ran three 1-second measurements. JMH 1.37 ran two 1-second warmups and
three 1-second measurements, one fork. Concurrent workloads use 8 or 16 threads.
Allocation, hit rate, and load count remain separate from the normalized rates.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation/workload         | Runtime | Result                                                                     |
|---------------------------------|---------|---------------------------------------------------------------------------:|
| Pluto mixed hit/update          | Go      | 370.782–376.364K ops/s; 206 B/op, 1 alloc/op                               |
| Guava mixed hit/update          | JMH     | 47.891M ops/s ± 4.205M                                                     |
| Pluto evicting write            | Go      | 767.460–770.416K ops/s; 511 B/op, 3 allocs/op                              |
| Guava evicting write            | JMH     | 12.929M ops/s ± 5.926M                                                     |
| Pluto forced miss + load        | Go      | 77.640–78.493K ops/s; 994 B/op, 10 allocs/op                               |
| Guava forced miss + load        | JMH     | 7.227M ops/s ± 1.379M                                                      |
| Pluto expiration + reload       | Go      | 3.270–3.306M ops/s; 880 B/op, 11 allocs/op                                 |
| Guava expiration + reload       | JMH     | 8.407M ops/s ± 0.838M                                                      |
| Pluto refresh workload          | Go      | 78.927–80.128K ops/s; 586–587 B/op, 8 allocs/op                            |
| Guava refresh workload          | JMH     | 6.951M ops/s ± 1.678M                                                      |
| Pluto LRU mixed hit/update      | Go      | 28.145–28.417M ops/s; 6 B/op, 0 allocs/op                                  |
| Pluto TinyLFU mixed hit/update  | Go      | 13.757–14.478M ops/s; 0 B/op, 0 allocs/op                                  |
| Pluto Zipf hits                 | Go      | 11.365–11.641M ops/s; 128 B/op, 1 alloc/op                                 |
| Guava Zipf hits                 | JMH     | 70.257M ops/s ± 5.643M                                                     |
| Pluto scan hits                 | Go      | 10.247–10.643M ops/s; 128 B/op, 1 alloc/op                                 |
| Guava scan hits                 | JMH     | 61.870M ops/s ± 5.673M                                                     |
| Pluto phase-change hits         | Go      | 11.391–11.711M ops/s; 128 B/op, 1 alloc/op                                 |
| Guava phase-change hits         | JMH     | 77.722M ops/s ± 4.159M                                                     |
| Pluto hits, 8 threads           | Go      | 3.174–3.245M ops/s; 128 B/op, 1 alloc/op                                   |
| Guava hits, 8 threads           | JMH     | 18.069M ops/s ± 0.905M                                                     |
| Pluto hits, 16 threads          | Go      | 2.376–2.420M ops/s; 128 B/op, 1 alloc/op                                   |
| Guava hits, 16 threads          | JMH     | 14.357M ops/s ± 0.207M                                                     |
| Pluto coalesced miss, 8 threads | Go      | 1.348–1.370M ops/s; 903 B/op, 10 allocs/op; 1.432–1.455M loads, ~1.2% hits |
| Guava coalesced miss, 8 threads | JMH     | 6.624M ops/s ± 0.830M                                                      |

The new cache uses a general mutex-protected policy core, while `lru_ts` and
`tiny_lfu_ts` have specialized/concurrent policies; these are separate design
baselines. Forced/coalesced misses deliberately include invalidation. Refresh uses
a direct executor in the matched workload. No single cross-runtime speed ratio is
calculated. Existing Caffeine comparisons remain independent baselines.

Raw files are `go-matched.txt`, `jmh-matched.json`, and `normalized.json`.
Reproduce the full suite with `make guava-bench`; ordinary tests never require Java.

## 2026-11-27: queue-based expiration and sharding

The table above records the original policy core. Profiling showed that 83% of
its CPU went to one design flaw, not to the language: every write (`Put`,
`GetWith`, `Len`, …) scanned the **entire** table for expired entries, even
when no expiration was configured, so a write cost O(n) — about 4,000 checks at
this corpus size. Three smaller costs followed. Every lookup heap-allocated a
probe entry (128 B/op). Every hit read the clock even when nothing used the
timestamp. All operations shared one global mutex.

The rewrite drains expired entries from the front of time-ordered queues
(Guava's write and access queues), so the cost is O(expired). Lookups use a
by-value table slot with a precomputed hash, so the probe stays on the stack.
The clock is read only when expiration or refresh is configured. The cache is
split into independently locked shards (Guava segments; default 16, small
bounded caches get fewer). Same machine and corpus, Go 1.27.0, three
1-second runs; raw output is `go-sharded.txt`. The Guava column repeats the
JMH results above (it was not re-run). `normalized.json` still holds the
original record until `make guava-bench` is re-run.

| Workload                | Pluto before      | Pluto after                          | Guava (JMH)        |
|-------------------------|------------------:|-------------------------------------:|-------------------:|
| Mixed hit/update        | 0.371–0.376M ops/s | 18.643–18.879M ops/s; 29 B/op, 0 allocs | 47.891M ± 4.205M |
| Evicting write          | 0.767–0.770M      | 11.744–11.933M; 144 B/op, 1 alloc     | 12.929M ± 5.926M   |
| Forced miss + load      | 0.078M            | 3.578–3.583M; 376 B/op, 6 allocs      | 7.227M ± 1.379M    |
| Expiration + reload     | 3.270–3.306M      | 5.467–5.501M; 376 B/op, 6 allocs      | 8.407M ± 0.838M    |
| Refresh workload        | 0.079–0.080M      | 5.115–5.198M; 344 B/op, 6 allocs      | 6.951M ± 1.678M    |
| Zipf hits               | 11.365–11.641M    | 32.300–35.537M; 0 allocs              | 70.257M ± 5.643M   |
| Scan hits               | 10.247–10.643M    | 32.216–32.404M; 0 allocs              | 61.870M ± 5.673M   |
| Phase-change hits       | 11.391–11.711M    | 38.358–39.494M; 0 allocs              | 77.722M ± 4.159M   |
| Hits, 8 threads         | 3.174–3.245M      | 15.995–16.305M; 0 allocs              | 18.069M ± 0.905M   |
| Hits, 16 threads        | 2.376–2.420M      | 13.282–13.387M; 0 allocs              | 14.357M ± 0.207M   |
| Coalesced miss, 8 thr.  | 1.348–1.370M      | 1.668–1.682M; 302 B/op, 5 allocs      | 6.624M ± 0.830M    |

Concurrent hits are now within about 10% of Guava, and evicting writes are
within Guava's error bar. Single-threaded hits remain about 2× behind. Each hit
still takes a shard mutex and moves the entry in the LRU list, and the key goes
through two calls via function values (hash and equality). Guava reads without
a lock and records accesses in a buffer that is drained later. That buffered
design (as in `tiny_lfu_ts`) is the remaining step if hit latency matters more
than exact per-shard LRU order. The coalesced-miss gap is the cost of a
channel, a `loadCall` and a `context.WithValue` per load.
