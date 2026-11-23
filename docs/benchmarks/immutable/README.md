# Phase 2 immutable-collection benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 4096 unique string keys, and 4096 seeded
uniform integer values. Multimaps use 64 keys. Construction benchmarks process
all 4096 inputs; iteration covers a full list; sorted subranges select the middle
half of the value universe. Go ran three 1-second measurements. JMH 1.37 ran two
1-second warmups and three 1-second measurements, one fork and one thread.
Allocations remain separate from the normalized comparison rates.

## Construction, copy, and lookup

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Family/workload     | Pluto Go                                  | Guava JMH                 |
|---------------------|------------------------------------------:|--------------------------:|
| List build          | 460.405–475.059K ops/s; 32792 B, 2 allocs | 1.391M ops/s ± 0.056M     |
| List copy-of        | 559.597–563.063M ops/s; 0 B, 0 allocs     | 452.735M ops/s ± 282.138M |
| List At             | 1.828–1.866G ops/s; 0 B, 0 allocs         | 484.086M ops/s            |
| List full iteration | 202.922–204.499K ops/s; 24 B, 2 allocs    | 0.725M ops/s ± 0.019M     |
| Set build           | 5.288–5.339K ops/s; 480361 B, 42 allocs   | 34.151K ops/s ± 13.799K   |
| Set contains        | 101.348–103.220M ops/s; 0 B, 0 allocs     | 307.828M ops/s ± 25.433M  |
| Map build           | 4.632–4.688K ops/s; 627169 B, 28 allocs   | 20.034K ops/s ± 1.865K    |
| Map get             | 53.476–55.218M ops/s; 0 B, 0 allocs       | 168.651M ops/s ± 8.271M   |
| Sorted set build    | 3.240–3.317K ops/s; 32816 B, 3 allocs     | 4.154K ops/s ± 0.221K     |
| Sorted set contains | 24.704–25.674M ops/s; 0 B, 0 allocs       | 35.480M ops/s ± 3.774M    |
| Sorted set subrange | 21.473–21.617M ops/s; 32 B, 1 alloc       | 20.153M ops/s ± 1.971M    |
| Sorted map build    | 27.488–27.917K ops/s; 98352 B, 3 allocs   | 18.119K ops/s ± 0.572K    |
| Sorted map get      | 15.218–15.404M ops/s; 0 B, 0 allocs       | 19.967M ops/s ± 3.661M    |
| Multiset build      | 5.053–5.123K ops/s; 570481 B, 42 allocs   | 9.911K ops/s ± 1.884K     |
| Multiset count      | 91.158–91.743M ops/s; 0 B, 0 allocs       | 234.254M ops/s ± 16.024M  |
| Multimap build      | 13.127–13.303K ops/s; 79112 B, 478 allocs | 31.545K ops/s ± 4.810K    |
| Multimap get        | 13.970–14.168M ops/s; 512 B, 1 alloc      | 277.371M ops/s ± 17.801M  |
| BiMap build         | 2.247–2.359K ops/s; 1118560 B, 56 allocs  | 12.257K ops/s ± 1.037K    |
| BiMap get           | 54.377–55.804M ops/s; 0 B, 0 allocs       | 174.608M ops/s ± 8.272M   |

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
