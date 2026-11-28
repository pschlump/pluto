# generic_graph benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| Sparse graph construction | 201.1–202.2 · 7879597 B, 41962 allocs | 609.8 ± 48.2 |
| Dense graph construction | 17.2–17.2 · 97673327 B, 29850 allocs | 103.9 ± 26.3 |
| Edge lookup | 15.42–15.57M · 0 B, 0 allocs | 26.09M ± 3.32M |
| Copy + edge removal | 500.5–503.7 · 4304158 B, 24655 allocs | 433.2 ± 40.3 |
| Adjacency snapshot + iteration | 6.30–6.40M · 238 B, 8 allocs | 6.37M ± 1.48M |
| BFS from first edge source | 1.13–1.14K · 1713126 B, 17556 allocs | 1.08K ± 0.21K |
| ValueGraph construction | 157.3–160.2 · 9626623 B, 42109 allocs | 582.8 ± 146.1 |
| ValueGraph lookup | 23.69–23.69M · 0 B, 0 allocs | 29.36M ± 1.16M |
| Network construction | 290.0–292.0 · 9203488 B, 25156 allocs | 348.3 ± 57.5 |

## Original record (2026-11-22)

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`. The shared corpus has
seed 1729, 4,096 nodes, 16,384 unique directed edges, and 10,000 mixed lookups.
Go ran three 1-second measurements; JMH 1.37 ran two 1-second warmups and three
1-second measurements, one fork/thread. Dense construction uses 512 nodes and
261,632 directed non-loop edges.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Workload                       | Pluto Go                                   | Guava JMH               |
|--------------------------------|-------------------------------------------:|------------------------:|
| Sparse graph construction      | 182.8–185.2 ops/s; 7.880 MB, 41,962 allocs | 609.825 ops/s ± 48.217  |
| Dense graph construction       | 13.5–13.8 ops/s; 97.67 MB, ~29,851 allocs  | 103.860 ops/s ± 26.341  |
| Edge lookup                    | 14.134–14.209M ops/s; 0 allocs             | 26.092M ops/s ± 3.316M  |
| Copy + edge removal            | 449.8–453.7 ops/s; 4.304 MB, 24,655 allocs | 433.169 ops/s ± 40.338  |
| Adjacency snapshot + iteration | 5.669–5.760M ops/s; 238 B, 8 allocs        | 6.374M ops/s ± 1.478M   |
| BFS from first edge source     | 954.2–963.4 ops/s; 1.713 MB, 17,556 allocs | 1.075K ops/s ± 207.888  |
| ValueGraph construction        | 142.0–144.8 ops/s; 9.627 MB, 42,109 allocs | 582.836 ops/s ± 146.133 |
| ValueGraph lookup              | 20.829–21.142M ops/s; 0 allocs             | 29.359M ops/s ± 1.156M  |
| Network construction           | 258.3–261.5 ops/s; 9.203 MB, 25,156 allocs | 348.328 ops/s ± 57.457  |

Retained-memory methods differ: Go uses GC-stable HeapAlloc; Java uses JOL object
graph size. Sparse Graph retained about 4.395 MB (214.6 B/node+edge), versus Guava
JOL 2,390,712 bytes. Dense Graph retained about 41.940 MB (160.0 B/node+edge),
versus Guava JOL 16,828,856 bytes. Treat these as separate measurements, not a
direct speed/space ratio.

Adapter costs are explicit: generic-to-dense conversion took 1.239–1.259 ms and
1.640 MB. Generic ValueGraph adaptation plus Dijkstra took 3.665–3.744 ms and
5.240 MB; direct Dijkstra on an already-built integer graph took 572.3–577.2 µs,
233,568 B, and 8 allocations. The adapter figure includes building the weighted
integer graph and running the algorithm; it is not presented as algorithm-only
time.

Raw files are `go-matched.txt`, `jmh-matched.json`, `normalized.json`, and
`graph-memory.json`. Reproduce the integrated suite with `make guava-bench`;
ordinary tests never require Java or JOL.
