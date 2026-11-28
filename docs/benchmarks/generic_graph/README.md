# generic_graph benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs on a quiet machine after the 2026-11-27 rewrites, shown
slowest–fastest, plus bytes and allocations per operation. Raw output is
`go-2026-11-27-current.txt`; the dated sections below give the causes and
before/after comparisons.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| Sparse graph construction | 956.7–965.1 · 2367317 B, 135 allocs | 609.8 ± 48.2 |
| Dense graph construction | 157.9–158.6 · 16824663 B, 7884 allocs | 103.9 ± 26.3 |
| Edge lookup | 28.28–30.10M · 0 B, 0 allocs | 26.09M ± 3.32M |
| Copy + edge removal | 7.62–7.63K · 1074120 B, 29 allocs | 433.2 ± 40.3 |
| Adjacency snapshot + iteration | 25.91–26.10M · 33 B, 0 allocs | 6.37M ± 1.48M |
| BFS from first edge source | 1.33–1.34K · 1277310 B, 16086 allocs | 1.08K ± 0.21K |
| ValueGraph construction | 774.3–774.7 · 3456107 B, 186 allocs | 582.8 ± 146.1 |
| ValueGraph lookup | 24.35–24.68M · 0 B, 0 allocs | 29.36M ± 1.16M |
| Network construction | 484.9–486.4 · 7635164 B, 332 allocs | 348.3 ± 57.5 |

## 2026-11-27: Flat adjacency storage, per-node index, native node map

### Results

Interleaved A/B on the same machine (Apple M4 Max, Go 1.27.0): each round ran
the pre-rewrite binary (commit 31a1d71's generic_graph code built with the new
`bench_test.go`) and then the new one, three rounds of 1-second runs. Other
agents were benchmarking concurrently (load average 9.5 falling to 5.9), so
the "before" numbers sit about 10% below the earlier "Current results" record
(`go-2026-11-27-current.txt`; e.g. sparse construction 201 then, 181–189 here). Raw output:
`go-2026-11-27-storage-interleaved.txt`; the solo pre-rewrite baseline is
`go-2026-11-27-storage-baseline.txt`.

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| Sparse graph construction | 180.7–188.8 · 7879729 B, 41962 allocs | 901.8–964.4 · 2367347 B, 135 allocs | 609.8 ± 48.2 |
| Dense graph construction | 13.7–15.2 · 97673476 B, 29851 allocs | 142.5–155.2 · 16824653 B, 7884 allocs | 103.9 ± 26.3 |
| Edge lookup | 13.88–14.62M · 0 B, 0 allocs | 27.05–28.78M · 0 B, 0 allocs | 26.09M ± 3.32M |
| Copy + edge removal | 411.9–456.6 · 4304162 B, 24655 allocs | 6.77–7.35K · 1074120 B, 29 allocs | 433.2 ± 40.3 |
| Adjacency snapshot + iteration | 5.31–5.92M · 238 B, 8 allocs | 23.84–25.87M · 33 B, 0 allocs | 6.37M ± 1.48M |
| BFS from first edge source | 1.04–1.06K · 1584879 B, 17540 allocs | 1.16–1.33K · 1277311 B, 16086 allocs | 1.08K ± 0.21K |
| ValueGraph construction | 145.0–150.6 · 9626634 B, 42109 allocs | 668.2–768.3 · 3456108 B, 186 allocs | 582.8 ± 146.1 |
| ValueGraph lookup | 22.20–23.45M · 0 B, 0 allocs | 22.03–24.88M · 0 B, 0 allocs | 29.36M ± 1.16M |
| Network construction | 258.4–270.7 · 9203499 B, 25156 allocs | 418.8–481.9 · 7635177 B, 332 allocs | 348.3 ± 57.5 |

With the JMH builders' capacity hints (`*Expected` benchmarks, new — see
harness notes):

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| Sparse graph construction, expectedNodeCount | 180.4–187.7 · 7548359 B, 41932 allocs | 1.05–1.17K · 1311651 B, 77 allocs | 609.8 ± 48.2 |
| Dense graph construction, expectedNodeCount | 13.9–14.7 · 97645290 B, 29832 allocs | 140.4–155.4 · 16742057 B, 7853 allocs | 103.9 ± 26.3 |
| ValueGraph construction, expectedNodeCount | 141.0–148.9 · 9295101 B, 42076 allocs | 848.8–947.5 · 2400407 B, 127 allocs | 582.8 ± 146.1 |
| Network construction, expected node + edge count | 286.5–304.4 · 6438484 B, 25067 allocs | 681.3–799.6 · 4077884 B, 155 allocs | 348.3 ± 57.5 |

Adapters, which read through `Edges()`/`EdgeValue`: dense adapter
818.4–852.3 → 1.20–1.31K ops/s (1639717 → 971038 B); ValueGraph Dijkstra
adapter 274.4–307.9 → 330.8–364.5 ops/s. Direct integer Dijkstra (control,
code unchanged) stayed at 1.59–1.85K vs 1.66–1.85K.

Retained memory (`BenchmarkGenericGraphMemory` / `DenseMemory`, GC-stable
HeapAlloc delta): sparse 4,394,976 → 1,287,160 bytes (214.6 → 62.85 B per
node+edge); dense 41,940,600 → 10,586,768 bytes (160.0 → 40.39 B per
node+edge). Guava's JOL figures are 2,390,712 and 16,828,856 bytes — a
different measurement method, but the Go graph is now the smaller of the two.

### Causes (profiled)

- **Per-node maps.** `Graph` kept `out`/`in` as `map[int]map[int]struct{}` plus
  `outOrder`/`inOrder` as `map[int][]int`: two small maps and two growing
  slices per node, each grown from empty. The allocation profile put 98.5% of
  sparse-build allocations in `connect`; that was the ~42K allocations.
- **A global edge map on top.** `edges map[pairKey]struct{}` duplicated
  membership already held by `out`. A stand-alone micro-benchmark of just
  inserting the 261,632 dense pairs into a `map[pairKey]int` cost ~12 ms and
  28 MB — more than Guava's whole dense build (9.6 ms). Edge membership
  needed a structure that is not a Go map.
- **Big-slice regrowth.** `edgeOrder` was one `[]pairKey`; above 256 elements
  `append` grows by 1.25x, so a 4.2 MB final slice allocated ~22 MB along the
  way (`alloc_space` profile), and the churn showed up as `runtime.madvise`
  in the CPU profile (re-committing scavenged spans).
- **Node lookup through hash_grow.** In the edge-lookup benchmark
  `hash_grow.(*HashTab).Search` was 55% of CPU, almost all on the line that
  compares `originalHash[h]` and then calls `eq(buckets[h], find)` — two
  parallel arrays (two cache lines per probe) plus an indirect call.
- **ValueGraph's second map.** `values map[pairKey]V` repeated every edge key
  again.
- **Network** used `map[int][]int` incidence lists (25K allocations).

### What changed

- `Graph.adj []adjacency`, indexed by node ID: exact, insertion-ordered `out`
  and `in` ID lists; undirected graphs store only `out` (the old predecessor
  list was provably the same sequence).
- No edge map: membership is a scan of `out[u]` up to 16 entries, then a
  per-node open-addressing `[]int32` index of positions (rebuilt on removal,
  since positions shift). An undirected edge sits in both endpoints' lists,
  so no reverse-orientation probe is needed. `EdgeCount` is a counter.
- ValueGraph values live in `[]V` indexed by a slot kept parallel to `out`
  (slots recycled; freed values zeroed) — one fewer map, one lookup path.
- `idArena` carves the first 4/8/16-entry list windows from shared chunks;
  `edgeLog` stores the edge order in fixed 2048-entry blocks; `Copy` packs
  all lists into one allocation.
- Natural constructors index nodes with a Go `map[N]int` behind closures
  (`nativeIndex`; no interface boxing); Func configs keep `hash_grow`.
- Network uses the same ID-indexed `adj` and arena.
- `Successors`/`Predecessors`/`Nodes` build their snapshot once (the
  `*Slice` forms return it directly instead of re-collecting an iterator).

### Harness notes

- **Go-side fix: BFS.** The Go benchmark collected the traversal into a
  slice; JMH counts lazily. Go now counts lazily too (both sides of the A/B
  above use the new harness; the old record's BFS figure includes the collect).
- **Capacity hints.** JMH builds with `expectedNodeCount(corpus.nodes)` (and
  `expectedEdgeCount` for the network); the Go rows use `NewGraph`/
  `NewValueGraph`/`NewNetwork`, which have no capacity parameter. The new
  `*Expected` benchmarks pass the same hints through the natural config
  internally. Public callers cannot do that today without giving up the native
  node index (`NewGraphFunc` with their own Equal/Hash uses hash_grow).
- **Java-side caveat (not changed).** `GraphBenchmark` increments its lookup
  index in a `@Setup(Level.Invocation)` method. JMH documents that
  per-invocation setup adds timestamping overhead that distorts
  nanosecond-scale benchmarks, so the Guava edge-lookup, ValueGraph-lookup and
  adjacency rates are probably pessimistic. Moving the index increment into
  the benchmark body would fix it; the shared Java harness was not edited here.
- Go's adjacency benchmark returns a snapshot copy (the Go contract); Guava
  iterates a live view. Go still does more work per call and is faster.

### Rejected or not pursued

- **Global map with packed keys.** In the stand-alone micro-benchmark a
  `map[uint64]int32` (u<<32|v) and a `map[[2]int32]int32` cut map bytes to 19
  and 14 MB but stayed at ~10–11 ms for the dense pair set; per-node Go maps
  were 7.8 ms with 8.7K allocations. All lost to the scan-plus-index design and
  the packed forms would cap node IDs at 32 bits. Not adopted.
- **Stage 1 (kept as a step, then superseded):** flat `adj` lists plus a global
  `map[pairKey]int` gave dense 22.5 ms / 59 MB; replacing the map with the
  per-node index gave 8.7 ms / 35 MB.
- **int32 node IDs in lists/indexes** would halve list and index bytes
  (dense: about 8 of the remaining 16.8 MB/op) but would put a cap on lifetime
  node insertions and add a new panic or error. Not done.
- Moving ValueGraph slots out of `adjacency` (96 → 72 bytes per node) was
  measured separately: sparse Graph construction 8–10% faster and
  3.13 → 2.37 MB/op, ValueGraph neutral. Kept.

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
