# multiset benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`; the HashMultiset and
TreeMultiset rows were re-measured after the 2026-11-27 table rewrite
(`go-2026-11-27-perf.txt`, taken under concurrent load, so they are
conservative — see the section below).
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| HashMultiset | 28.41–28.76M · 0 B, 0 allocs | 67.89M ± 7.74M |
| TreeMultiset | 5.42–5.51M · 4 B, 0 allocs | 6.08M ± 1.53M |
| Baseline: plain map of counts (Java: HashMap) | 37.89–38.10M · 0 B, 0 allocs | 52.87M ± 8.26M |

## 2026-11-27: one-probe count tables

Rates are operations per second on the matched corpus. "Go before" is the
earlier 2026-11-27 current-results run; "Go after" is `go-2026-11-27-perf.txt`
(three 1-second runs, load average 7–10 from other agents' benchmarks, so it
understates the change). The interleaved old/new A/B run under identical load,
`go-2026-11-27-perf-ab.txt` (lines prefixed `old`/`new`, old built from the
parent commit), is the cleaner ratio: HashMultiset 41.3–44.0 → 33.4–35.5 ns/op,
LinkedHashMultiset 65.0–66.4 → 34.7–37.7 ns/op, TreeMultiset 214.9–231.0 →
181.2–192.4 ns/op, while the plain-map baseline stayed at 28.1–29.3 ns/op.

| Workload | Go before | Go after | Guava (Java) |
|---|---:|---:|---:|
| HashMultiset | 25.34–25.45M · 0 B, 0 allocs | 28.41–28.76M · 0 B, 0 allocs | 67.89M ± 7.74M |
| TreeMultiset | 4.92–4.94M · 4 B, 0 allocs | 5.42–5.51M · 4 B, 0 allocs | 6.08M ± 1.53M |
| LinkedHashMultiset (Java: LinkedHashMultiset) | 16.13–16.25M · 4 B, 0 allocs | 27.22–29.90M · 0 B, 0 allocs | 64.82M ± 31.19M |
| Baseline: plain map of counts (same session) | 37.89–38.10M | 33.75–37.09M | 52.87M ± 8.26M |

Causes, from CPU profiles of the matched benchmark:

- **Every mutation hashed and probed twice.** Add/Remove/SetCount called
  `Count` (hash_grow `Search`) and then `Insert`/`Delete`, which hashed and
  probed again; with the benchmark's trailing `Count` that was three hashes
  per operation where the Go-map baseline does two. `hash_grow` also keeps
  hashes and elements in two parallel arrays (two cache lines per probe) and
  the multiset wrapped the caller's eq/hash in a second closure layer.
  HashMultiset now owns a private open-addressing `table` (`multiset/table.go`):
  one slot holds hash, value and count, `find` returns the slot index, and the
  count is updated in place or inserted at the returned empty slot.
- **LinkedHashMultiset paid a Go map plus a heap node per value.**
  `linked_hash_map` is `map[uint64][]*node` with a separate allocation per
  entry (the 4 B/op) and pointer-chasing on every lookup, again twice per
  mutation. The new `linkedTable` threads the insertion-order list through the
  inline slots by index (backward-shift deletion rewrites the moved entry's
  neighbours), so it is allocation-free in steady state.
- **TreeMultiset descended twice on Add of a present value** (findNode, then
  the recursive rebalancing insert that rewrote every link on the path). A
  present value is now updated in place after one descent.

Remaining gap: the matched HashMultiset now does the same work as the Go-map
baseline (two hash computations, one probe run each); the profile is the
string switch on the operation name (shared with the baseline), the
`maphash.Comparable` call through the stored hash function value (the
runtime's string hasher, same as Go maps use), and the probe loads. TreeMultiset
time is dominated by `compareOrdered`, which makes two `cmpstring` calls when
the probe is not less than the node (`a < b`, then `b < a`) — the
repository-wide comparator idiom.

Harness: Go and JMH do the same work per operation (one mutation by op name,
then `count`); the Go side stores the count in a package variable, Java
returns it. No harness change was made.

Rejected: an equality-first comparator for TreeMultiset (`a == b`, then
`a < b`) measured 184.6–191.7 ns/op against 176.0–181.1 for the current one,
and it would change NaN ordering for floats; reverted.

## Original record (2026-11-22)

Recorded 2026-11-22 on an Apple M4 Max, darwin/arm64, Go 1.27.0, and
OpenJDK 11.0.28. Guava source commit:
`36e3995499d6c3b74b1207d362fa9e19915d2589`.

The matched workload uses the committed 10,000-operation corpus, seed 1729,
4096 distinct string keys, and 45% add / 25% remove / 20% count / 10% setCount.
Go ran three 1-second measurements with `testing.B`; JMH 1.37 ran two 1-second
warmups and three 1-second measurements, one fork and one thread. Allocations
and memory remain separate from the normalized comparison rates. No cross-runtime
speed ratio is inferred from these numbers.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Implementation     | Runtime | Result                                    |
|--------------------|---------|------------------------------------------:|
| Pluto HashMultiset | Go      | 21.358–22.026M ops/s; 0 B/op, 0 allocs/op |
| Pluto TreeMultiset | Go      | 4.861–4.895M ops/s; 4 B/op, 0 allocs/op   |
| Go map model       | Go      | 36.792–37.341M ops/s; 0 B/op, 0 allocs/op |
| Guava HashMultiset | JMH     | 67.886M ops/s ± 7.739M                    |
| Guava TreeMultiset | JMH     | 6.075M ops/s ± 1.529M                     |
| Java HashMap model | JMH     | 52.867M ops/s ± 8.256M                    |

The additional Go workloads use 4096 keys for hit/miss Count, 1024 distinct keys
for entry/occurrence iteration and algebra, and 4096 distinct keys for retained
memory. Duplicate ratios are 1, 4, and 16. Sum, Union, Intersection, and Minus
are measured separately. These are single 100ms exploratory runs, not stable
performance claims.

At 4096 distinct int keys, GC-stable retained-memory differences were about
196.8 KiB for Pluto hash, 196.6 KiB for Pluto tree, and 147.8 KiB for the Go map.
Per-occurrence memory falls as the duplicate ratio rises. These are Go heap
measurements only and are not numerically comparable with Java/JOL measurements.

Raw files:

- `go-matched.txt` and `jmh-matched.json`: matched workload output.
- `normalized.json`: matched metrics with complete environment metadata.
- `go-extra.txt`: count, iteration, algebra, and retained-memory output.

Reproduce the matched run with `make guava-bench`. Reproduce the additional Go
run with:

```sh
go test ./multiset -run '^$' \
  -bench 'BenchmarkMultiset(Count|Iteration|Algebra|Memory)$' \
  -benchmem -benchtime=100ms -count=1
```
