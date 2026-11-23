# Phase 5 generic Bloom benchmark and accuracy record

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`. Go ran three 1-second
measurements; JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork/thread. Values are UTF-8 `value-%04d` strings. Put creates a 100,000-entry
shape and inserts 1,000 values per operation. Other filters contain 1,000 values.

| Workload | Pluto Go | Guava JMH |
|---|---:|---:|
| Create + 1,000 puts | 85.00–85.67 µs; 314945 B, 7002 allocs | 14.808K ops/s ± 1.007K |
| Membership hit | 71.92–72.73 ns; 192 B, 7 allocs | 19.088M ops/s ± 4.206M |
| Membership miss | 77.83–79.47 ns; 192 B, 7 allocs | 16.739M ops/s ± 2.535M |
| Copy + merge | 206.5–207.7 ns; 1280 B, 1 alloc | 1.026M ops/s ± 0.092M |
| Guava compact serialization | 1.015–1.030 µs; 4096 B, 9 allocs | 1.074M ops/s ± 0.105M |

The differential fixture is 1,000 insertions at design FPP 0.03. Every bit and
compact-stream byte matches Guava. A separate 10,000-member/10,000-disjoint-
nonmember test at design FPP 0.01 rejects measured FPP above 0.025 and approximate
count outside 9,000–11,000. Existing `bloom` retains separate frozen hashes and
serialization; its established Add/MayContain/Merge benchmarks remain baselines,
not interchangeable strategy measurements. The retained `legacy-bloom-go.txt`
run measured Add 23.94–24.02 ns, hit 22.58–22.67 ns, miss 33.56–33.84 ns, and
merge 47.07–48.72 µs, all zero-allocation, at its documented 1M/16-byte workload.

Raw files: `go-matched.txt`, `jmh-matched.json`; normalized metrics are in
`normalized.json`. Reproduce with `make guava-bench`.
