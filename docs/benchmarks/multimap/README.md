# Phase 1b multimap benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 256 string keys, 16 values per key, and
10,000 operations: 45% put, 20% remove, 15% get, 10% putAll, and 10%
containsEntry. Go ran three 1-second measurements. JMH 1.37 ran two 1-second
warmups and three 1-second measurements, one fork and one thread. The run also
includes Phase 0/1a fixtures; normalized per-metric workload metadata separates
those corpora.

| Implementation                      | Runtime | Result                                  |
|-------------------------------------|---------|----------------------------------------:|
| Pluto ListMultimap                  | Go      | 1223–1236 ns/op; about 6.3 KiB/op       |
| Pluto SetMultimap                   | Go      | 150.8–160.2 ns/op; 222 B/op, 1 alloc/op |
| Pluto SortedSetMultimap             | Go      | 136.4–139.1 ns/op; 108 B/op             |
| Go `map[string][]string`            | Go      | 1108–1130 ns/op; 32 B/op                |
| Go `map[string]map[string]struct{}` | Go      | 40.72–41.00 ns/op; 0 B/op, 0 allocs/op  |
| Guava ArrayListMultimap             | JMH     | 0.630M ops/s ± 1.871M                   |
| Guava HashMultimap                  | JMH     | 36.392M ops/s ± 20.070M                 |
| Guava TreeMultimap                  | JMH     | 10.395M ops/s ± 4.551M                  |
| Java list-map baseline              | JMH     | 0.623M ops/s ± 1.787M                   |
| Java set-map baseline               | JMH     | 38.991M ops/s ± 25.826M                 |

List-heavy streams grow duplicate collections, so PutAll's snapshot/rebuild
cost is visible. SetPutAll also rebuilds the per-key set to preserve atomicity;
the Go map baseline is substantially faster on this stream. These are workload
observations, not single-ratio claims about general performance. JMH confidence
errors are large for several cases and are retained rather than hidden.

Raw files:

- `go-matched.txt`: all Go matched measurements and allocation metrics.
- `jmh-matched.json`: all JMH measurements.
- `normalized.json`: normalized metrics with environment and per-workload
  capacity, distribution, and seed.

Reproduce with `make guava-bench`.
