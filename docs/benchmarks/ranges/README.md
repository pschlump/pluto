# Phase 3 range and ordered-navigation benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed range corpus has seed 1729, a 4096-point universe, and 10,000
operations: 50% add/put, 20% remove, and 30% point queries, with mixed uniform
and clustered ranges. Iteration fixtures contain 512 disjoint ranges. Go ran
three 1-second measurements. JMH 1.37 ran two 1-second warmups and three
1-second measurements, one fork and one thread. Latency, throughput, and
allocations remain separate metrics.

## Range collections

| Implementation/workload         | Runtime | Result                                           |
|---------------------------------|---------|-------------------------------------------------:|
| Pluto RangeSet mixed stream     | Go      | 2244–2258 ns/op; 5241–5242 B/op, 1 alloc/op      |
| Guava TreeRangeSet mixed stream | JMH     | 4.394M ops/s ± 0.321M                            |
| Pluto RangeMap mixed stream     | Go      | 12.09–12.25 µs/op; 26497–26509 B/op, 2 allocs/op |
| Guava TreeRangeMap mixed stream | JMH     | 2.892M ops/s ± 0.308M                            |
| Pluto RangeSet iteration        | Go      | 1877–1938 ns/op; 18432 B/op, 1 alloc/op          |
| Guava TreeRangeSet iteration    | JMH     | 599.122K ops/s ± 60.371K                         |
| Pluto RangeMap iteration        | Go      | 2384–2418 ns/op; 27264 B/op, 1 alloc/op          |
| Guava TreeRangeMap iteration    | JMH     | 312.869K ops/s ± 11.498K                         |

The Pluto implementations use compact sorted slices with O(r) mutation and point
lookup; Guava uses tree indexes. These measurements describe this workload, not
an asymptotic equivalence claim.

## Positioned AVL range query

The fixture contains 4096 ordered integers and returns the 12 values in closed
range `[1024,1035]`. The filtered baseline is the former full-prefix walk.

| Implementation | Runtime | Result |
|---|---|---:|
| Pluto positioned AVL range | Go | 169.4–172.7 ns/op; 280 B/op, 5 allocs/op |
| Pluto filtered baseline | Go | 2941–2957 ns/op; 24 B/op, 2 allocs/op |
| Guava TreeSet subSet iteration | JMH | 18.214M ops/s ± 2.239M |

The direct before/after comparison is approximately 17x here and, more
importantly, the new implementation is O(log n + results) rather than O(n).

Raw `go-matched.txt`, `jmh-matched.json`, and `normalized.json` include mixed,
iteration, and positioned-range workloads. Reproduce all comparisons with
`make guava-bench`.
