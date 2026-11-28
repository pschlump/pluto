# evicting_queue benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| Ring, mixed operations | 277.39–295.16M · 0 B, 0 allocs | 108.14M ± 15.06M |
| Ring iteration, 256 elements | 13.77–13.81M · 0 B, 0 allocs | 8.25M ± 1.09M |
| Baseline: deque + manual eviction (Java: ArrayDeque) | 52.27–52.47M · 31 B, 1 alloc | 96.64M ± 49.89M |
| Baseline: deque iteration (Java: ArrayDeque) | 2.94–2.95M · 40 B, 2 allocs | 8.30M ± 0.86M |

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The committed corpus has seed 1729, capacity 256, and 10,000 operations: 50%
push, 20% pop, 10% peek, and 20% four-element bulk push. Go ran three 1-second
measurements. JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork and one thread. Iteration fixtures contain 256 integers. Allocations
remain separate from the normalized rates; no cross-runtime speed ratio is claimed.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation/workload               | Runtime | Result                                      |
|---------------------------------------|---------|--------------------------------------------:|
| Pluto ring, mixed operations          | Go      | 117.398–118.343M ops/s; 0 B/op, 0 allocs/op |
| Pluto dqueue + manual eviction        | Go      | 48.591–49.652M ops/s; 31 B/op, 1 alloc/op   |
| Guava EvictingQueue, mixed operations | JMH     | 108.144M ops/s ± 15.056M                    |
| Java ArrayDeque + manual eviction     | JMH     | 96.642M ops/s ± 49.887M                     |
| Pluto ring iteration                  | Go      | 6.536–6.575M ops/s; 0 B/op, 0 allocs/op     |
| Pluto dqueue iteration                | Go      | 2.811–2.861M ops/s; 24 B/op, 2 allocs/op    |
| Guava EvictingQueue iteration         | JMH     | 8.249M ops/s ± 1.085M                       |
| Java ArrayDeque iteration             | JMH     | 8.301M ops/s ± 0.862M                       |

The linked deque baseline pays for node allocation; the ring reuses fixed
storage. Iteration measurements cover the full logical contents, not one item.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: the complete shared-harness run.
- `normalized.json`: environment, workload, and per-implementation metrics.

Reproduce with `make guava-bench`. Ordinary tests never invoke Java or Maven.

## 2026-11-27: division-free ring

Every element access computed `(head+index) % capacity`, and `%` compiles
to an integer division. That division was the largest cost in `Push`, `Pop`,
`Slice` and especially `All`. Java's `ArrayDeque` iterator advances with a
compare-and-reset. The ring now wraps with one compare-and-subtract (`wrap`).
`All`, `Slice` and `Clone` treat the contents as at most two contiguous runs
of the backing array (`segments`), so iteration is two plain slice loops: no
division, no bounds checks, and the whole iterator inlines into the caller's
loop. `Clone` also no longer builds an intermediate `Slice`, and `Equal` no
longer allocates two snapshots.

The iteration benchmark was also corrected to match the Java harness. Java
sums into a local `int` and returns it, while the Go benchmark added each
element into a package-level variable, which put a store and reload on every
element. The corrected harness alone accounts for 149 → 136 ns; the code
change accounts for 136 → 67 ns. Same machine, corpus and Go version; raw
output is `go-modfree.txt`. Guava figures repeat the JMH run above.

| Workload                         | Before                | After                 | Guava (JMH)        |
|----------------------------------|----------------------:|----------------------:|-------------------:|
| Ring, mixed operations           | 117.398–118.343M ops/s | 253.6–282.3M ops/s   | 108.144M ± 15.056M |
| Ring iteration (256 elements)    | 6.536–6.575M          | 14.736–14.835M        | 8.249M ± 1.085M    |

Both rows have 0 B/op and 0 allocs/op. Iteration is now about 1.8× Guava's
rate, partly because Java iterates boxed `Integer`s and must unbox each one.
The dqueue baselines are unchanged by this work.
