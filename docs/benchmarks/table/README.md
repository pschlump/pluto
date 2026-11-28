# table benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`; the mixed, row,
column and transpose rows were re-measured after the 2026-11-27 index rework
below (raw output `go-2026-11-27-perf-after.txt`).
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| HashTable mixed (Java: HashBasedTable) | 21.52–22.21M · 0 B, 0 allocs | 31.85M ± 11.19M |
| TreeTable mixed (Java: TreeBasedTable) | 10.74–10.89M · 6 B, 0 allocs | 11.12M ± 4.49M |
| ArrayTable mixed | 40.78–41.41M · 0 B, 0 allocs | 55.95M ± 19.88M |
| HashTable row | 2.18–2.34M · 3200 B, 1 alloc | 293.17M ± 28.80M |
| HashTable column | 1.40–1.49M · 3200 B, 1 alloc | 6.08M ± 0.84M |
| TreeTable row | 1.88–2.05M · 3200 B, 1 alloc | 39.94M ± 0.96M |
| TreeTable column | 581.73–598.44K · 3200 B, 1 alloc | 1.29M ± 0.10M |
| ArrayTable row | 2.25–2.50M · 3200 B, 1 alloc | 286.79M ± 23.22M |
| ArrayTable column | 2.26–2.41M · 3200 B, 1 alloc | 293.70M ± 17.09M |
| HashTable transpose | 10.39–10.66K · 357361 B, 135 allocs | 11.99K ± 0.84K |
| TreeTable transpose | 7.57–7.89K · 477530 B, 162 allocs | 4.37K ± 0.40K |
| ArrayTable transpose | 99.86–109.29K · 69792 B, 3 allocs | 22.30K ± 0.60K |
| Baseline: nested maps | 27.93–28.33M · 0 B, 0 allocs | 50.84M ± 8.13M |
| Baseline: flat matrix + presence bitmap | 264.34–303.67M · 0 B, 0 allocs | 199.26M ± 16.45M |

Guava's row and column accessors return live views, while Pluto's return fresh copies (the allocations in the Go column). The access figures therefore compare different contracts.

The ArrayTable mixed row now removes with `DeleteAt(rowIndex, columnIndex)`,
matching the JMH benchmark's index-based `set(rowIndex, columnIndex, null)`;
earlier Go figures removed by key (two extra hash lookups per remove).

## 2026-11-27: private indexes, one-allocation snapshots, bulk transposes

Same machine and toolchain. Before and after were built from `31a1d71` and
this change and run interleaved (old, new, old, new, ...), three 1-second
runs each, both with the corrected ArrayTable harness; other agents were
benchmarking concurrently (load average 6.6-14.0 during the runs, so treat
small differences as noise). Raw output: `go-2026-11-27-perf-before.txt`,
`go-2026-11-27-perf-after.txt`.

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| HashTable mixed | 20.42–20.52M · 0 B, 0 allocs | 21.52–22.21M · 0 B, 0 allocs | 31.85M ± 11.19M |
| TreeTable mixed | 8.24–8.33M · 25 B, 0 allocs | 10.74–10.89M · 6 B, 0 allocs | 11.12M ± 4.49M |
| ArrayTable mixed | 29.01–29.61M · 0 B, 0 allocs | 40.78–41.41M · 0 B, 0 allocs | 55.95M ± 19.88M |
| HashTable row | 1.85–2.20M · 3200 B, 1 alloc | 2.18–2.34M · 3200 B, 1 alloc | 293.17M ± 28.80M |
| HashTable column | 449.84–530.22K · 7504 B, 7 allocs | 1.40–1.49M · 3200 B, 1 alloc | 6.08M ± 0.84M |
| TreeTable row | 1.27–1.28M · 3344 B, 6 allocs | 1.88–2.05M · 3200 B, 1 alloc | 39.94M ± 0.96M |
| TreeTable column | 346.26–359.84K · 7648 B, 12 allocs | 581.73–598.44K · 3200 B, 1 alloc | 1.29M ± 0.10M |
| ArrayTable row | 904.98–972.76K · 7504 B, 7 allocs | 2.25–2.50M · 3200 B, 1 alloc | 286.79M ± 23.22M |
| ArrayTable column | 911.58–978.47K · 7504 B, 7 allocs | 2.26–2.41M · 3200 B, 1 alloc | 293.70M ± 17.09M |
| HashTable transpose | 3.65–3.91K · 706253 B, 781 allocs | 10.39–10.66K · 357361 B, 135 allocs | 11.99K ± 0.84K |
| TreeTable transpose | 1.84–1.90K · 279300 B, 4682 allocs | 7.57–7.89K · 477530 B, 162 allocs | 4.37K ± 0.40K |
| ArrayTable transpose | 8.73–9.04K · 90800 B, 41 allocs | 99.86–109.29K · 69792 B, 3 allocs | 22.30K ± 0.60K |

With the old harness (key-based remove) the old ArrayTable mixed figure was
24.15–26.02M; about 4M of the 29M-to-41M step is the harness, the rest is the
code.

Causes, from CPU and allocation profiles:

- HashTable sat on `hash_grow` behind adapter closures: every lookup called a
  wrapper closure that called the caller's function, and `Put`/`Delete` hashed
  each key twice (a `Search` for the old value, then `Insert`/`Delete`).
  A private linear-probing index (cached hash per slot, backward-shift
  deletion, Fibonacci home slot, the caller's functions called directly) hashes
  once per write. The remaining gap to nested Go maps (27-28M) is the
  `maphash.Comparable` call chain behind `NewHashTable`'s hash function: a Go
  map hashes a string key in about half the time.
- Column snapshots grew their result by `append` from empty (7 allocations,
  7504 B) and re-hashed the column key once per row. Now: the column key is
  hashed once and probed with the cached hash in every row; the result is
  preallocated (`min(rows, cells, 1024)` cells).
- ArrayTable looked its universe up through the same adapter-closure hash
  index. `NewArrayTable` now keeps its universe in built-in maps behind a
  position closure (a Go map lookup is ~4.5 ns against ~10 ns); the universe
  is immutable, so Clone, Transpose, Replace and JSON share it instead of
  rebuilding two indexes. Transpose is now a direct matrix transpose (3
  allocations). Row and Column count the present cells first and copy into an
  exact slice.
- TreeTable sat on `avl_tree`, which allocates a node on every `Insert` -
  including replacements - recurses through a heap closure, walks `All()` with
  a recursive closure (5 allocations per row snapshot), and needs a second
  descent to report the old value. A private AVL (`table/tree_index.go`)
  descends once with a fixed-size stack, replaces in place, and traverses
  without closures. Transpose and Clone no longer insert cell by cell: Clone
  copies each row structurally into one node slab, and Transpose regroups the
  cells column-major (a merge cursor against the previous row's columns, then
  a counting sort) and builds each transposed row as a perfectly balanced tree
  in one slab - 162 allocations instead of 4682. It allocates more bytes than
  before (temporary column-major arrays) but far fewer objects.

Retained memory (`BenchmarkTableMemory`, 4096 cells, bytes retained after GC):

| Table | Before | After |
|---|---:|---:|
| HashTable | 459,888 (112.3/cell) | 422,040 (103.0/cell) |
| TreeTable | 337,744 (82.5/cell) | 335,664 (81.9/cell) |
| ArrayTable | 84,728 (20.7/cell) | 83,112 (20.3/cell) |
| Nested maps (baseline) | 388,736 | 388,736 |
| Flat matrix (baseline) | 71,880 | 71,880 |

Harness fixes: the Go ArrayTable mixed benchmark removed by key while JMH
removes by index; it now calls `DeleteAt`. The row/column benchmarks still
differ by contract (Guava's `row(k).size()` obtains a live view and reads its
size; Pluto copies the cells), and that difference is intentional.

Rejected experiments:

- Struct-of-arrays slots for the hash index (hashes in their own `[]uint64`):
  mixed HashTable within noise (40.2-41.7 ns against 41.2-42.3 ns
  interleaved), Transpose 5-9% slower and 204 instead of 135 allocations (two
  arrays per index). Kept array-of-structs.
- A three-way natural comparator for `NewTreeTable` (`<` then `==` instead of
  `cmp.Compare`'s `<` then `>`): TreeTable mixed 96.9-98.3 ns against
  88.3-95.7 ns for `cmp.Compare`, interleaved. Kept `cmp.Compare`.

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, 64 string rows, 64 string columns, and
10,000 operations: 40% put, 20% remove, and 40% get. Go ran three 1-second
measurements. JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork and one thread. Allocation and memory remain separate from the
normalized comparison rates. JMH row/column calls return Guava views, while Pluto returns
allocated snapshots; this is an intentional API-cost comparison, not a claim of
identical mechanics. Transpose benchmarks copy on both sides.

## Mixed operation stream

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation                   | Runtime | Result                                      |
|----------------------------------|---------|--------------------------------------------:|
| Pluto HashTable                  | Go      | 15.972–16.923M ops/s; 0 B/op, 0 allocs/op   |
| Pluto TreeTable                  | Go      | 8.237–8.340M ops/s; 25 B/op, 0 allocs/op    |
| Go nested maps                   | Go      | 27.115–27.541M ops/s; 0 B/op, 0 allocs/op   |
| Pluto ArrayTable                 | Go      | 19.550–21.763M ops/s; 0 B/op, 0 allocs/op   |
| Go flat matrix + presence bitmap | Go      | 272.926–298.686M ops/s; 0 B/op, 0 allocs/op |
| Guava HashBasedTable             | JMH     | 31.852M ops/s ± 11.192M                     |
| Guava TreeBasedTable             | JMH     | 11.121M ops/s ± 4.487M                      |
| Java nested maps                 | JMH     | 50.836M ops/s ± 8.127M                      |
| Guava ArrayTable                 | JMH     | 55.954M ops/s ± 19.884M                     |
| Java flat matrix                 | JMH     | 199.263M ops/s ± 16.445M                    |

## Row, column, and transpose snapshots


|--------------------------------|---------|-----------------------------------------------:|
| Pluto HashTable row            | Go      | 2.164–2.247M ops/s; 3200 B/op, 1 alloc/op       |
| Pluto HashTable column         | Go      | 400.320–540.249K ops/s; 7504 B/op, 7 allocs/op        |
| Pluto TreeTable row            | Go      | 1.300–1.316M ops/s; 3344 B/op, 6 allocs/op      |
| Pluto TreeTable column         | Go      | 364.697–366.166K ops/s; 7648 B/op, 12 allocs/op       |
| Pluto ArrayTable row           | Go      | 979.432–999.001K ops/s; 7504 B/op, 7 allocs/op        |
| Pluto ArrayTable column        | Go      | 969.932–977.517K ops/s; 7504 B/op, 7 allocs/op        |
| Pluto HashTable transpose      | Go      | 3.034–3.352K ops/s; 706253 B/op, 781 allocs/op  |
| Pluto TreeTable transpose      | Go      | 1.847–1.868K ops/s; 279299 B/op, 4682 allocs/op |
| Pluto ArrayTable transpose     | Go      | 7.305–7.994K ops/s; 90800 B/op, 41 allocs/op    |
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
