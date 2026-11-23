# Phase 1e min-max priority-queue benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

All corpora use seed 1729 and 10,000 uniform integer operations. The both-ended
stream is 45% push, 20% popMin, 20% popMax, 10% peekMin, and 5% peekMax. The
one-ended stream is 60% push, 25% popMin, and 15% peekMin. The bounded fixture
retains the least 256 elements. Go ran three 1-second measurements. JMH 1.37 ran
two 1-second warmups and three 1-second measurements, one fork and one thread.
Go latency and allocations remain separate from JMH throughput.

| Implementation/workload              | Runtime | Result                                    |
|--------------------------------------|---------|------------------------------------------:|
| Pluto min-max heap, both ends        | Go      | 96.93–97.05 ns/op; 2 B/op, 0 allocs/op    |
| Go lazy two-heap alternative         | Go      | 145.7–148.6 ns/op; 89–97 B/op, 1 alloc/op |
| Guava MinMaxPriorityQueue, both ends | JMH     | 10.532M ops/s ± 0.870M                    |
| Java lazy two-heap alternative       | JMH     | 6.666M ops/s ± 9.103M                     |
| Pluto bounded min-max heap           | Go      | 52.20–52.94 ns/op; 0 B/op, 0 allocs/op    |
| Guava bounded MinMaxPriorityQueue    | JMH     | 19.715M ops/s ± 2.937M                    |
| Pluto min-max heap, one end          | Go      | 77.77–78.22 ns/op; 14 B/op, 0 allocs/op   |
| Pluto normal heap, one end           | Go      | 51.14–51.44 ns/op; 14 B/op, 0 allocs/op   |
| Guava MinMaxPriorityQueue, one end   | JMH     | 13.251M ops/s ± 7.252M                    |
| Java PriorityQueue, one end          | JMH     | 21.674M ops/s ± 13.277M                   |

The two-heap baseline stores identity-stamped entries in min and max heaps and
uses lazy discard cleanup. The normal-heap measurements support the API warning:
one-ended workloads favor a normal heap. These are workload records, not claims
that Go and Java runtimes have directly equivalent allocation mechanics.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.

Reproduce with `make guava-bench`. Ordinary tests never invoke Java or Maven.
