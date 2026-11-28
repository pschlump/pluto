# Phase 3 range and ordered-navigation benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed range corpus has seed 1729, a 4096-point universe, and 10,000
operations: 50% add/put, 20% remove, and 30% point queries, with mixed uniform
and clustered ranges. Iteration fixtures contain 512 disjoint ranges. Go ran
three 1-second measurements. JMH 1.37 ran two 1-second warmups and three
1-second measurements, one fork and one thread. Allocations remain separate
from the normalized comparison rates.

## Range collections

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation/workload         | Runtime | Result                                              |
|---------------------------------|---------|----------------------------------------------------:|
| Pluto RangeSet mixed stream     | Go      | 442.870–445.633K ops/s; 5241–5242 B/op, 1 alloc/op  |
| Guava TreeRangeSet mixed stream | JMH     | 4.394M ops/s ± 0.321M                               |
| Pluto RangeMap mixed stream     | Go      | 81.633–82.713K ops/s; 26497–26509 B/op, 2 allocs/op |
| Guava TreeRangeMap mixed stream | JMH     | 2.892M ops/s ± 0.308M                               |
| Pluto RangeSet iteration        | Go      | 515.996–532.765K ops/s; 18432 B/op, 1 alloc/op      |
| Guava TreeRangeSet iteration    | JMH     | 599.122K ops/s ± 60.371K                            |
| Pluto RangeMap iteration        | Go      | 413.565–419.463K ops/s; 27264 B/op, 1 alloc/op      |
| Guava TreeRangeMap iteration    | JMH     | 312.869K ops/s ± 11.498K                            |

The Pluto implementations use compact sorted slices with O(r) mutation and point
lookup; Guava uses tree indexes. These measurements describe this workload, not
an asymptotic equivalence claim.

## Positioned AVL range query

The fixture contains 4096 ordered integers and returns the 12 values in closed
range `[1024,1035]`. The filtered baseline is the former full-prefix walk.

| Implementation | Runtime | Result |
|---|---|---:|
| Pluto positioned AVL range | Go | 5.790–5.903M ops/s; 280 B/op, 5 allocs/op |
| Pluto filtered baseline | Go | 338.181–340.020K ops/s; 24 B/op, 2 allocs/op |
| Guava TreeSet subSet iteration | JMH | 18.214M ops/s ± 2.239M |

The direct before/after comparison is approximately 17x here and, more
importantly, the new implementation is O(log n + results) rather than O(n).

Raw `go-matched.txt`, `jmh-matched.json`, and `normalized.json` include mixed,
iteration, and positioned-range workloads. Reproduce all comparisons with
`make guava-bench`.

## 2026-11-27: binary-searched windows replace whole-slice rebuilds

Same machine (Apple M4 Max, darwin/arm64, Go 1.27.0), same corpus and
fixtures, `go test ./range_set/ ./range_map/ -run='^$' -bench=. -benchmem
-count=3`. "Before" is a fresh baseline of the previous commit taken the same
day (it agrees with the 2026-11-22 record to within 2%). Go ops/s are the
reciprocal of ns/op, slowest to fastest. The Guava JMH figures are repeated
from the 2026-11-22 record, not re-run (Maven was unavailable).

| Workload | Before (Go) | After (Go) | Guava JMH (record) |
|---|---:|---:|---:|
| RangeSet mixed stream | 452.1–456.2K ops/s; 5241–5242 B/op, 1 alloc/op | 6.798–7.215M ops/s; 11 B/op, 0 allocs/op | 4.394M ops/s ± 0.321M |
| RangeMap mixed stream | 80.98–81.10K ops/s; 26499–26502 B/op, 2 allocs/op | 3.839–3.857M ops/s; 11 B/op, 0 allocs/op | 2.892M ops/s ± 0.308M |
| RangeSet iteration | 518.4–524.4K ops/s; 18432 B/op, 1 alloc/op | 517.9–539.1K ops/s; 18432 B/op, 1 alloc/op | 599.122K ops/s ± 60.371K |
| RangeMap iteration | 409.8–415.3K ops/s; 27264 B/op, 1 alloc/op | 413.7–416.0K ops/s; 27264 B/op, 1 alloc/op | 312.869K ops/s ± 11.498K |

The mixed streams are ~15.6x (set) and ~47x (map) faster and now beat the
Guava record (~1.6x and ~1.3x). Iteration is unchanged, as expected: `All`
and `Entries` are documented snapshots and copy the slice.

**Cause.** The memory profile put 99% of allocated bytes in `Add`/`Remove`
(set) and `Put`/`Remove` (map): every mutation allocated a fresh slice of
len(values)+1 and copied every stored range through it, even when the
argument touched nothing; `Put` then ran `sort.SliceStable` (reflection
swapper, O(r log r)) over the whole result; and `Contains`/`Get` scanned
linearly. At steady state the corpus holds ~177 stored ranges (32 B each) and
~627 map entries (48 B each — the map never coalesces); with 70% of operations
mutating, plus allocation size-class rounding, that is the observed 5.2 KB and
26.5 KB per operation. The map's larger entry count is also why it stays
slower than the set after the fix (its tail memmove is ~5x longer).

**Fix.** Stored ranges are sorted and disjoint, so their upper bounds ascend
with their lower bounds and both halves of `IsConnected` are monotone over the
slice: the ranges an argument touches form one contiguous run, located by two
binary searches. Only that run is rebuilt (into a small stack buffer) and
spliced in place with `slices.Replace`; point lookup and `Encloses` are a
single binary search. Mutations are now O(log r) comparisons plus an O(r)
worst-case memmove with no per-call allocation; lookups are O(log r). The
remaining 11 B/op is the benchmark harness's own `ranges.ClosedOpen` call (the
`cmp.Compare[T]` function value it stores escapes with the range), not the
collections. In the after-profile of the map stream, the residual cost is
roughly a fifth memmove of the tail (typed copy — entries hold pointers), a
fifth binary search (comparator calls are indirect by design), and the rest
fragment construction.

**Correctness.** New `reference_test.go` files keep the original whole-slice
algorithms as differential oracles and compare stored ranges (bounds and
open/closed types) and every query on 6000-step fixed-seed random streams:
continuous, sparse (up to 319 stored entries), discrete, int8 discrete at both
extrema, and a reversed comparator. The oracle exposed a pre-existing bug: in a
finite discrete domain, removing (or overwriting) up to the maximum from an
unbounded range stored the canonical remainder `(max,max)` — an empty range —
which violated "empty ranges are no-ops" and the sorted-disjoint invariant.
Empty fragments are now dropped (regression tests
`TestRangeSetDiscreteRemoveAtMaximumStoresNoEmptyRange` and
`TestRangeMapDiscretePutAtMaximumStoresNoEmptyEntry`).

**Harness.** The Go benchmarks incremented a package-global counter inside the
loop (once per element in the iteration benchmarks) while the JMH methods
count into a local and return it. They now count into a local and publish once.

**Rejected.** Removing Add's two successor-adjacency loops (unreachable for
canonical ranges) measured 130–131 ns/op against 130–134 ns/op with them —
noise — so they stay, keeping Add identical to the `adjacent` predicate.

Raw output: `go-before-binary-search-2026-11-27.txt` and
`go-binary-search-2026-11-27.txt`.
