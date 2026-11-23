# Phase 1c bimap benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729 and 4096 string keys/values: 20% put,
10% forcePut, 10% deleteKey, 10% deleteValue, 25% getValue, and 25% getKey.
Go ran three 1-second measurements. JMH 1.37 ran two 1-second warmups and three
1-second measurements, one fork and one thread. Allocation and memory remain
separate from the normalized comparison rates.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation/workload            | Runtime | Result                                      |
|------------------------------------|---------|--------------------------------------------:|
| Pluto HashBiMap mixed stream       | Go      | 23.441–24.372M ops/s; 0 B/op, 0 allocs/op   |
| Go two-map mixed stream            | Go      | 38.895–39.573M ops/s; 0 B/op, 0 allocs/op   |
| Guava HashBiMap mixed stream       | JMH     | 20.239M ops/s ± 2.300M                      |
| Java two-map mixed stream          | JMH     | 58.503M ops/s ± 8.930M                      |
| Pluto 4096-entry inverse iteration | Go      | 141.764–143.493K ops/s; 0 B/op, 0 allocs/op |
| Guava 4096-entry inverse iteration | JMH     | 28,872 ops/s ± 6,249                        |

The Go retained-memory comparison used 4096 associations and GC-stable heap
differences. Pluto HashBiMap retained about 787 KiB (192.2 bytes/association),
while two Go maps retained about 789 KiB (192.6 bytes/association), including
the string objects retained by each approach. These Go heap measurements are
not directly comparable with Java memory measurements.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: matched stream and inverse iteration.
- `normalized.json`: environment and per-workload capacity/distribution/seed.
- `go-memory.txt`: retained-memory and construction-allocation measurements.

Reproduce matched measurements with `make guava-bench` and memory with:

```sh
go test ./bimap -run '^$' -bench 'BenchmarkBiMapMemory$' \
  -benchmem -benchtime=100ms -count=1
```
