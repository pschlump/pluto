# Phase 4 cache benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 4096 keys, and 10,000 operations (80% get,
20% put). Separate keys provide Zipf, scan, and phase-change workloads. Eviction
capacity is 256. Expiration/refresh advances a fake clock by 2 ns against a 1 ns
policy. Go ran three 1-second measurements. JMH 1.37 ran two 1-second warmups and
three 1-second measurements, one fork. Concurrent workloads use 8 or 16 threads.
Latency, throughput, allocation, hit rate, and load count remain separate.

| Implementation/workload | Runtime | Result |
|---|---|---:|
| Pluto mixed hit/update | Go | 2657–2697 ns/op; 206 B/op, 1 alloc/op |
| Guava mixed hit/update | JMH | 47.891M ops/s ± 4.205M |
| Pluto evicting write | Go | 1298–1303 ns/op; 511 B/op, 3 allocs/op |
| Guava evicting write | JMH | 12.929M ops/s ± 5.926M |
| Pluto forced miss + load | Go | 12.74–12.88 µs/op; 994 B/op, 10 allocs/op |
| Guava forced miss + load | JMH | 7.227M ops/s ± 1.379M |
| Pluto expiration + reload | Go | 302.5–305.8 ns/op; 880 B/op, 11 allocs/op |
| Guava expiration + reload | JMH | 8.407M ops/s ± 0.838M |
| Pluto refresh workload | Go | 12.48–12.67 µs/op; 586–587 B/op, 8 allocs/op |
| Guava refresh workload | JMH | 6.951M ops/s ± 1.678M |
| Pluto LRU mixed hit/update | Go | 35.19–35.53 ns/op; 6 B/op, 0 allocs/op |
| Pluto TinyLFU mixed hit/update | Go | 69.07–72.69 ns/op; 0 B/op, 0 allocs/op |
| Pluto Zipf hits | Go | 85.90–87.99 ns/op; 128 B/op, 1 alloc/op |
| Guava Zipf hits | JMH | 70.257M ops/s ± 5.643M |
| Pluto scan hits | Go | 93.96–97.59 ns/op; 128 B/op, 1 alloc/op |
| Guava scan hits | JMH | 61.870M ops/s ± 5.673M |
| Pluto phase-change hits | Go | 85.39–87.79 ns/op; 128 B/op, 1 alloc/op |
| Guava phase-change hits | JMH | 77.722M ops/s ± 4.159M |
| Pluto hits, 8 threads | Go | 308.2–315.1 ns/op; 128 B/op, 1 alloc/op |
| Guava hits, 8 threads | JMH | 18.069M ops/s ± 0.905M |
| Pluto hits, 16 threads | Go | 413.3–420.9 ns/op; 128 B/op, 1 alloc/op |
| Guava hits, 16 threads | JMH | 14.357M ops/s ± 0.207M |
| Pluto coalesced miss, 8 threads | Go | 729.9–741.8 ns/op; 903 B/op, 10 allocs/op; 1.432–1.455M loads, ~1.2% hits |
| Guava coalesced miss, 8 threads | JMH | 6.624M ops/s ± 0.830M |

The new cache uses a general mutex-protected policy core, while `lru_ts` and
`tiny_lfu_ts` have specialized/concurrent policies; these are separate design
baselines. Forced/coalesced misses deliberately include invalidation. Refresh uses
a direct executor in the matched workload. No single cross-runtime speed ratio is
calculated. Existing Caffeine comparisons remain independent baselines.

Raw files are `go-matched.txt`, `jmh-matched.json`, and `normalized.json`.
Reproduce the full suite with `make guava-bench`; ordinary tests never require Java.
