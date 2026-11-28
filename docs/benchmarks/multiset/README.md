# multiset benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| HashMultiset | 25.34–25.45M · 0 B, 0 allocs | 67.89M ± 7.74M |
| TreeMultiset | 4.92–4.94M · 4 B, 0 allocs | 6.08M ± 1.53M |
| Baseline: plain map of counts (Java: HashMap) | 37.89–38.10M · 0 B, 0 allocs | 52.87M ± 8.26M |

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The matched workload uses the committed 10,000-operation corpus, seed 1729,
4096 distinct string keys, and 45% add / 25% remove / 20% count / 10% setCount.
Go ran three 1-second measurements with `testing.B`; JMH 1.37 ran two 1-second
warmups and three 1-second measurements, one fork and one thread. Allocations
and memory remain separate from the normalized comparison rates. No cross-runtime
speed ratio is inferred from these numbers.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation     | Runtime | Result                                    |
|--------------------|---------|------------------------------------------:|
| Pluto HashMultiset | Go      | 21.358–22.026M ops/s; 0 B/op, 0 allocs/op |
| Pluto TreeMultiset | Go      | 4.861–4.895M ops/s; 4 B/op, 0 allocs/op   |
| Go map model       | Go      | 36.792–37.341M ops/s; 0 B/op, 0 allocs/op |
| Guava HashMultiset | JMH     | 67.886M ops/s ± 7.739M                    |
| Guava TreeMultiset | JMH     | 6.075M ops/s ± 1.529M                     |
| Java HashMap model | JMH     | 52.867M ops/s ± 8.256M                    |

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
