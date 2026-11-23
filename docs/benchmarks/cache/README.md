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
