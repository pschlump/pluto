# Phase 9 completeness benchmark record

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
