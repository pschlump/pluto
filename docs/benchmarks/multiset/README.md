# Phase 1a multiset benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The matched workload uses the committed 10,000-operation corpus, seed 1729,
4096 distinct string keys, and 45% add / 25% remove / 20% count / 10% setCount.
Go ran three 1-second measurements with `testing.B`; JMH 1.37 ran two 1-second
warmups and three 1-second measurements, one fork and one thread. Throughput,
latency, allocations, and memory are separate measurements. No cross-runtime
speed ratio is inferred from these numbers.

| Implementation | Runtime | Result |
|---|---|---:|
| Pluto HashMultiset | Go | 45.40–46.82 ns/op; 0 B/op, 0 allocs/op |
| Pluto TreeMultiset | Go | 204.3–205.7 ns/op; 4 B/op, 0 allocs/op |
| Go map model | Go | 26.78–27.18 ns/op; 0 B/op, 0 allocs/op |
| Guava HashMultiset | JMH | 67.886M ops/s ± 7.739M |
| Guava TreeMultiset | JMH | 6.075M ops/s ± 1.529M |
| Java HashMap model | JMH | 52.867M ops/s ± 8.256M |

The additional Go workloads use 4096 keys for hit/miss Count, 1024 distinct keys
for entry/occurrence iteration and algebra, and 4096 distinct keys for retained
memory. Duplicate ratios are 1, 4, and 16. Sum, Union, Intersection, and Minus
are measured separately. These are single 100ms exploratory runs, not stable
performance claims.

At 4096 distinct int keys, GC-stable retained-memory differences were about
196.8 KiB for Pluto hash, 196.6 KiB for Pluto tree, and 147.8 KiB for the Go map.
Per-occurrence memory falls as the duplicate ratio rises. These are Go heap
measurements only and are not numerically comparable with Java/JOL measurements.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: matched workload output.
- `normalized.json`: matched metrics with complete environment metadata.
- `go-extra.txt`: count, iteration, algebra, and retained-memory output.

Reproduce the matched run with `make guava-bench`. Reproduce the additional Go
run with:

```sh
go test ./multiset -run '^$' \
  -bench 'BenchmarkMultiset(Count|Iteration|Algebra|Memory)$' \
  -benchmem -benchtime=100ms -count=1
```
