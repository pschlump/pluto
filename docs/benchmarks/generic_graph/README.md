# Phase 6 generic graph benchmark record

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`. The shared corpus has
seed 1729, 4,096 nodes, 16,384 unique directed edges, and 10,000 mixed lookups.
Go ran three 1-second measurements; JMH 1.37 ran two 1-second warmups and three
1-second measurements, one fork/thread. Dense construction uses 512 nodes and
261,632 directed non-loop edges.

| Workload | Pluto Go | Guava JMH |
|---|---:|---:|
| Sparse graph construction | 5.40–5.47 ms; 7.880 MB, 41,962 allocs | 609.825 ops/s ± 48.217 |
| Dense graph construction | 72.48–73.99 ms; 97.67 MB, ~29,851 allocs | 103.860 ops/s ± 26.341 |
| Edge lookup | 70.38–70.75 ns; 0 allocs | 26.092M ops/s ± 3.316M |
| Copy + edge removal | 2.204–2.223 ms; 4.304 MB, 24,655 allocs | 433.169 ops/s ± 40.338 |
| Adjacency snapshot + iteration | 173.6–176.4 ns; 238 B, 8 allocs | 6.374M ops/s ± 1.478M |
| BFS from first edge source | 1.038–1.048 ms; 1.713 MB, 17,556 allocs | 1.075K ops/s ± 207.888 |
| ValueGraph construction | 6.904–7.044 ms; 9.627 MB, 42,109 allocs | 582.836 ops/s ± 146.133 |
| ValueGraph lookup | 47.30–48.01 ns; 0 allocs | 29.359M ops/s ± 1.156M |
| Network construction | 3.824–3.872 ms; 9.203 MB, 25,156 allocs | 348.328 ops/s ± 57.457 |

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
