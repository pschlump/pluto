# Phase 1e evicting-queue benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, capacity 256, and 10,000 operations: 50%
push, 20% pop, 10% peek, and 20% four-element bulk push. Go ran three 1-second
measurements. JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork and one thread. Iteration fixtures contain 256 integers. Go latency and
allocations are separate from JMH throughput; no cross-runtime speed ratio is
claimed.

| Implementation/workload | Runtime | Result |
|---|---|---:|
| Pluto ring, mixed operations | Go | 8.450–8.518 ns/op; 0 B/op, 0 allocs/op |
| Pluto dqueue + manual eviction | Go | 20.14–20.58 ns/op; 31 B/op, 1 alloc/op |
| Guava EvictingQueue, mixed operations | JMH | 108.144M ops/s ± 15.056M |
| Java ArrayDeque + manual eviction | JMH | 96.642M ops/s ± 49.887M |
| Pluto ring iteration | Go | 152.1–153.0 ns/op; 0 B/op, 0 allocs/op |
| Pluto dqueue iteration | Go | 349.5–355.7 ns/op; 24 B/op, 2 allocs/op |
| Guava EvictingQueue iteration | JMH | 8.249M ops/s ± 1.085M |
| Java ArrayDeque iteration | JMH | 8.301M ops/s ± 0.862M |

The linked deque baseline pays for node allocation; the ring reuses fixed
storage. Iteration measurements cover the full logical contents, not one item.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.

Reproduce with `make guava-bench`. Ordinary tests never invoke Java or Maven.
