# Phase 1d table benchmark record

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 64 string rows, 64 string columns, and
10,000 operations: 40% put, 20% remove, and 40% get. Go ran three 1-second
measurements. JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork and one thread. Latency, throughput, allocation, and memory remain
separate metrics. JMH row/column calls return Guava views, while Pluto returns
allocated snapshots; this is an intentional API-cost comparison, not a claim of
identical mechanics. Transpose benchmarks copy on both sides.

## Mixed operation stream

| Implementation                   | Runtime | Result                                  |
|----------------------------------|---------|----------------------------------------:|
| Pluto HashTable                  | Go      | 59.09–62.61 ns/op; 0 B/op, 0 allocs/op  |
| Pluto TreeTable                  | Go      | 119.9–121.4 ns/op; 25 B/op, 0 allocs/op |
| Go nested maps                   | Go      | 36.31–36.88 ns/op; 0 B/op, 0 allocs/op  |
| Pluto ArrayTable                 | Go      | 45.95–51.15 ns/op; 0 B/op, 0 allocs/op  |
| Go flat matrix + presence bitmap | Go      | 3.348–3.664 ns/op; 0 B/op, 0 allocs/op  |
| Guava HashBasedTable             | JMH     | 31.852M ops/s ± 11.192M                 |
| Guava TreeBasedTable             | JMH     | 11.121M ops/s ± 4.487M                  |
| Java nested maps                 | JMH     | 50.836M ops/s ± 8.127M                  |
| Guava ArrayTable                 | JMH     | 55.954M ops/s ± 19.884M                 |
| Java flat matrix                 | JMH     | 199.263M ops/s ± 16.445M                |

## Row, column, and transpose snapshots


|--------------------------------|---------|-----------------------------------------------:|
| Pluto HashTable row            | Go      | 445.0–462.2 ns/op; 3200 B/op, 1 alloc/op       |
| Pluto HashTable column         | Go      | 1851–2498 ns/op; 7504 B/op, 7 allocs/op        |
| Pluto TreeTable row            | Go      | 760.0–769.4 ns/op; 3344 B/op, 6 allocs/op      |
| Pluto TreeTable column         | Go      | 2731–2742 ns/op; 7648 B/op, 12 allocs/op       |
| Pluto ArrayTable row           | Go      | 1001–1021 ns/op; 7504 B/op, 7 allocs/op        |
| Pluto ArrayTable column        | Go      | 1023–1031 ns/op; 7504 B/op, 7 allocs/op        |
| Pluto HashTable transpose      | Go      | 298.3–329.6 µs/op; 706253 B/op, 781 allocs/op  |
| Pluto TreeTable transpose      | Go      | 535.3–541.3 µs/op; 279299 B/op, 4682 allocs/op |
| Pluto ArrayTable transpose     | Go      | 125.1–136.9 µs/op; 90800 B/op, 41 allocs/op    |
| Guava HashBasedTable row       | JMH     | 293.168M ops/s ± 28.796M                       |
| Guava HashBasedTable column    | JMH     | 6.082M ops/s ± 0.841M                          |
| Guava TreeBasedTable row       | JMH     | 39.938M ops/s ± 0.955M                         |
| Guava TreeBasedTable column    | JMH     | 1.286M ops/s ± 0.096M                          |
| Guava ArrayTable row           | JMH     | 286.792M ops/s ± 23.224M                       |
| Guava ArrayTable column        | JMH     | 293.704M ops/s ± 17.092M                       |
| Guava HashBasedTable transpose | JMH     | 11,987 ops/s ± 839                             |
| Guava TreeBasedTable transpose | JMH     | 4,368 ops/s ± 402                              |
| Guava ArrayTable transpose     | JMH     | 22,301 ops/s ± 596                             |

All row/column/transpose fixtures contain 4096 cells. Sparse mixed streams create
and remove empty rows; dense operations keep the complete 64-by-64 universe.
Guava ArrayTable does not implement remove, so its mixed benchmark uses indexed
`set(null)` and does not query cell-set size afterward.

## Retained memory

The Go comparison used 4096 populated cells and GC-stable heap differences.
HashTable retained 459,888 bytes (112.3 per cell), TreeTable 337,744 (82.46),
ArrayTable 84,728 (20.69), nested maps 388,736 (94.91), and the flat string
matrix plus presence bitmap 71,880 (17.55). The flat baseline does not retain row
or column key maps. These measurements include table-owned keys and value slice
headers but not Java heap usage; Go and Java memory methods are not directly
comparable.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: all matched Go/JMH measurements.
- `normalized.json`: environment and per-workload capacity/distribution/seed.
- `go-memory.txt`: GC-stable retained-memory measurements.

Reproduce matched measurements with `make guava-bench` and memory with:

```sh
go test ./table -run '^$' -bench 'BenchmarkTableMemory$' \
  -benchmem -benchtime=100ms -count=1
```
