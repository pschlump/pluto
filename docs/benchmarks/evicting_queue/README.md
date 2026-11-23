# Phase 1e evicting-queue benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, capacity 256, and 10,000 operations: 50%
push, 20% pop, 10% peek, and 20% four-element bulk push. Go ran three 1-second
measurements. JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork and one thread. Iteration fixtures contain 256 integers. Allocations
remain separate from the normalized rates; no cross-runtime speed ratio is claimed.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation/workload               | Runtime | Result                                      |
|---------------------------------------|---------|--------------------------------------------:|
| Pluto ring, mixed operations          | Go      | 117.398–118.343M ops/s; 0 B/op, 0 allocs/op |
| Pluto dqueue + manual eviction        | Go      | 48.591–49.652M ops/s; 31 B/op, 1 alloc/op   |
| Guava EvictingQueue, mixed operations | JMH     | 108.144M ops/s ± 15.056M                    |
| Java ArrayDeque + manual eviction     | JMH     | 96.642M ops/s ± 49.887M                     |
| Pluto ring iteration                  | Go      | 6.536–6.575M ops/s; 0 B/op, 0 allocs/op     |
| Pluto dqueue iteration                | Go      | 2.811–2.861M ops/s; 24 B/op, 2 allocs/op    |
| Guava EvictingQueue iteration         | JMH     | 8.249M ops/s ± 1.085M                       |
| Java ArrayDeque iteration             | JMH     | 8.301M ops/s ± 0.862M                       |

The linked deque baseline pays for node allocation; the ring reuses fixed
storage. Iteration measurements cover the full logical contents, not one item.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.

Reproduce with `make guava-bench`. Ordinary tests never invoke Java or Maven.
