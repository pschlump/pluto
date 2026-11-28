# min_max_pq benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| Min-max heap, both ends | 10.56–10.60M · 2 B, 0 allocs | 10.53M ± 0.87M |
| Bounded min-max heap | 19.46–19.58M · 0 B, 0 allocs | 19.72M ± 2.94M |
| Min-max heap, one end | 13.37–13.39M · 13 B, 0 allocs | 13.25M ± 7.25M |
| Baseline: normal heap, one end (Java: PriorityQueue) | 20.06–20.10M · 14 B, 0 allocs | 21.67M ± 13.28M |
| Baseline: lazy two-heap, both ends | 7.44–7.47M · 89 B, 1 alloc | 6.67M ± 9.10M |

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

All corpora use seed 1729 and 10,000 uniform integer operations. The both-ended
stream is 45% push, 20% popMin, 20% popMax, 10% peekMin, and 5% peekMax. The
one-ended stream is 60% push, 25% popMin, and 15% peekMin. The bounded fixture
retains the least 256 elements. Go ran three 1-second measurements. JMH 1.37 ran
two 1-second warmups and three 1-second measurements, one fork and one thread.
Allocations remain separate from the normalized comparison rates.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation/workload              | Runtime | Result                                     |
|--------------------------------------|---------|-------------------------------------------:|
| Pluto min-max heap, both ends        | Go      | 10.304–10.317M ops/s; 2 B/op, 0 allocs/op  |
| Go lazy two-heap alternative         | Go      | 6.729–6.863M ops/s; 89–97 B/op, 1 alloc/op |
| Guava MinMaxPriorityQueue, both ends | JMH     | 10.532M ops/s ± 0.870M                     |
| Java lazy two-heap alternative       | JMH     | 6.666M ops/s ± 9.103M                      |
| Pluto bounded min-max heap           | Go      | 18.889–19.157M ops/s; 0 B/op, 0 allocs/op  |
| Guava bounded MinMaxPriorityQueue    | JMH     | 19.715M ops/s ± 2.937M                     |
| Pluto min-max heap, one end          | Go      | 12.784–12.858M ops/s; 14 B/op, 0 allocs/op |
| Pluto normal heap, one end           | Go      | 19.440–19.554M ops/s; 14 B/op, 0 allocs/op |
| Guava MinMaxPriorityQueue, one end   | JMH     | 13.251M ops/s ± 7.252M                     |
| Java PriorityQueue, one end          | JMH     | 21.674M ops/s ± 13.277M                    |

The two-heap baseline stores identity-stamped entries in min and max heaps and
uses lazy discard cleanup. The normal-heap measurements support the API warning:
one-ended workloads favor a normal heap. These are workload records, not claims
that Go and Java runtimes have directly equivalent allocation mechanics.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.

Reproduce with `make guava-bench`. Ordinary tests never invoke Java or Maven.
