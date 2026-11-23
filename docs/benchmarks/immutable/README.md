# Phase 2 immutable-collection benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 4096 unique string keys, and 4096 seeded
uniform integer values. Multimaps use 64 keys. Construction benchmarks process
all 4096 inputs; iteration covers a full list; sorted subranges select the middle
half of the value universe. Go ran three 1-second measurements. JMH 1.37 ran two
1-second warmups and three 1-second measurements, one fork and one thread.
Latency, throughput, and allocations remain separate metrics.

## Construction, copy, and lookup

| Family/workload     | Pluto Go                                | Guava JMH                 |
|---------------------|----------------------------------------:|--------------------------:|
| List build          | 2.105–2.172 µs/op; 32792 B, 2 allocs    | 1.391M ops/s ± 0.056M     |
| List copy-of        | 1.776–1.787 ns/op; 0 B, 0 allocs        | 452.735M ops/s ± 282.138M |
| List At             | 0.536–0.547 ns/op; 0 B, 0 allocs        | 484.086M ops/s            |
| List full iteration | 4.890–4.928 µs/op; 24 B, 2 allocs       | 0.725M ops/s ± 0.019M     |
| Set build           | 187.3–189.1 µs/op; 480361 B, 42 allocs  | 34.151K ops/s ± 13.799K   |
| Set contains        | 9.688–9.867 ns/op; 0 B, 0 allocs        | 307.828M ops/s ± 25.433M  |
| Map build           | 213.3–215.9 µs/op; 627169 B, 28 allocs  | 20.034K ops/s ± 1.865K    |
| Map get             | 18.11–18.70 ns/op; 0 B, 0 allocs        | 168.651M ops/s ± 8.271M   |
| Sorted set build    | 301.5–308.6 µs/op; 32816 B, 3 allocs    | 4.154K ops/s ± 0.221K     |
| Sorted set contains | 38.95–40.48 ns/op; 0 B, 0 allocs        | 35.480M ops/s ± 3.774M    |
| Sorted set subrange | 46.26–46.57 ns/op; 32 B, 1 alloc        | 20.153M ops/s ± 1.971M    |
| Sorted map build    | 35.82–36.38 µs/op; 98352 B, 3 allocs    | 18.119K ops/s ± 0.572K    |
| Sorted map get      | 64.92–65.71 ns/op; 0 B, 0 allocs        | 19.967M ops/s ± 3.661M    |
| Multiset build      | 195.2–197.9 µs/op; 570481 B, 42 allocs  | 9.911K ops/s ± 1.884K     |
| Multiset count      | 10.90–10.97 ns/op; 0 B, 0 allocs        | 234.254M ops/s ± 16.024M  |
| Multimap build      | 75.17–76.18 µs/op; 79112 B, 478 allocs  | 31.545K ops/s ± 4.810K    |
| Multimap get        | 70.58–71.58 ns/op; 512 B, 1 alloc       | 277.371M ops/s ± 17.801M  |
| BiMap build         | 423.9–445.1 µs/op; 1118560 B, 56 allocs | 12.257K ops/s ± 1.037K    |
| BiMap get           | 17.92–18.39 ns/op; 0 B, 0 allocs        | 174.608M ops/s ± 8.272M   |

Pluto Multimap `Get` returns an allocated snapshot, while Guava returns a live
view; this is intentionally an API-cost comparison. Copy-of returns the existing
immutable value on both sides. Do not infer a single cross-runtime speed ratio
from latency and throughput measured by different frameworks.

## Retained memory

Go uses GC-stable heap-allocation differences with corpus inputs retained before
the baseline. Java uses JOL `GraphLayout.totalSize`, which includes reachable
shared input objects. The methods and resulting numbers are not directly
comparable. Detailed results are in `go-memory.txt` and `java-memory.json`.

| Family     | Go retained bytes (per input) | Guava JOL bytes (per input) |
|------------|------------------------------:|----------------------------:|
| List       | 32768 (8.00)                  | 81104 (19.80)               |
| Set        | 224128 (54.72)                | 67968 (16.59)               |
| Map        | 360672 (88.05)                | 392424 (95.81)              |
| Sorted set | 32816 (8.01)                  | 51608 (12.60)               |
| Sorted map | 98352 (24.01)                 | 294208 (71.83)              |
| Multiset   | 254208 (62.06)                | 129792 (31.69)              |
| Multimap   | 40256 (9.83)                  | 88328 (21.56)               |
| BiMap      | 655840 (160.1)                | 437232 (106.7)              |

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.
- `go-memory.txt` and `java-memory.json`: retained-memory measurements.

Reproduce matched measurements and JOL output with `make guava-bench`. Run Go
memory separately:

```sh
go test ./immutable -run '^$' -bench 'BenchmarkImmutableMemory$' \
  -benchmem -benchtime=100ms -count=1
```
