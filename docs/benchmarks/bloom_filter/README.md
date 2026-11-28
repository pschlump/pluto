# Phase 5 generic Bloom benchmark and accuracy record

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`. Go ran three 1-second
measurements; JMH 1.37 ran two 1-second warmups and three 1-second measurements,
one fork/thread. Values are UTF-8 `value-%04d` strings. Put creates a 100,000-entry
shape and inserts 1,000 values per operation. Other filters contain 1,000 values.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Workload                    | Pluto Go                                    | Guava JMH              |
|-----------------------------|--------------------------------------------:|-----------------------:|
| Create + 1,000 puts         | 11.673–11.765K ops/s; 314945 B, 7002 allocs | 14.808K ops/s ± 1.007K |
| Membership hit              | 13.749–13.904M ops/s; 192 B, 7 allocs       | 19.088M ops/s ± 4.206M |
| Membership miss             | 12.583–12.849M ops/s; 192 B, 7 allocs       | 16.739M ops/s ± 2.535M |
| Copy + merge                | 4.815–4.843M ops/s; 1280 B, 1 alloc         | 1.026M ops/s ± 0.092M  |
| Guava compact serialization | 970.874–985.222K ops/s; 4096 B, 9 allocs    | 1.074M ops/s ± 0.105M  |

The differential fixture is 1,000 insertions at design FPP 0.03. Every bit and
compact-stream byte matches Guava. A separate 10,000-member/10,000-disjoint-
nonmember test at design FPP 0.01 rejects measured FPP above 0.025 and approximate
count outside 9,000–11,000. Existing `bloom` retains separate frozen hashes and
serialization; its established Add/MayContain/Merge benchmarks remain baselines,
not interchangeable strategy measurements. The retained `legacy-bloom-go.txt`
run measured Add 23.94–24.02 ns, hit 22.58–22.67 ns, miss 33.56–33.84 ns, and
merge 47.07–48.72 µs, all zero-allocation, at its documented 1M/16-byte workload.

Raw files: `go-matched.txt`, `jmh-matched.json`; normalized metrics are in
`normalized.json`. Reproduce with `make guava-bench`.

## 2026-11-27: Allocation-free membership checks and puts

Same machine and Go version; Guava figures are repeated from the 2026-11-22
record above (not re-run — Maven was unavailable). Go was measured as three
interleaved before/after rounds (`-count=1` each) while the machine carried a
load average of roughly 6–10, so absolute rates sit a little below the quiet
2026-11-22 run; the before/after ratio is the meaningful figure. "Before" is
commit `7bbed0c` source with the harness-fixed `bench_test.go` described below.
Raw output: `go-2026-11-27-alloc-free.txt`.

| Workload                    | Go before                                     | Go after                                    | Guava JMH (2026-11-22) |
|-----------------------------|----------------------------------------------:|--------------------------------------------:|-----------------------:|
| Create + 1,000 puts         | 11.41–12.49K ops/s; 314945 B, 7002 allocs     | 29.52–31.49K ops/s; 123017 B, 2 allocs      | 14.808K ops/s ± 1.007K |
| Membership hit              | 13.13–14.31M ops/s; 192 B, 7 allocs           | 46.99–49.58M ops/s; 0 B, 0 allocs           | 19.088M ops/s ± 4.206M |
| Membership miss             | 12.67–13.20M ops/s; 192 B, 7 allocs           | 35.98–37.13M ops/s; 0 B, 0 allocs           | 16.739M ops/s ± 2.535M |
| Copy + merge (+ mightContain) | 3.469–3.712M ops/s; 1472 B, 8 allocs        | 4.281–4.634M ops/s; 1280 B, 1 alloc         | 1.026M ops/s ± 0.092M  |
| Guava compact serialization (+ copy) | 874.9–919.1K ops/s; 5376 B, 10 allocs | 2.297–2.414M ops/s; 3888 B, 4 allocs      | 1.074M ops/s ± 0.105M  |

`bloom_filter_ts` (new benchmark file mirroring the plain workloads):

| Workload                    | Go before                                 | Go after                               |
|-----------------------------|------------------------------------------:|---------------------------------------:|
| Create + 1,000 puts         | 10.82–11.30K ops/s; 314977 B, 7003 allocs | 23.77–24.94K ops/s; 123051 B, 3 allocs |
| Membership hit              | 11.96–13.76M ops/s; 192 B, 7 allocs       | 41.17–42.59M ops/s; 0 B, 0 allocs      |
| Membership miss             | 11.06–12.54M ops/s; 192 B, 7 allocs       | 30.67–31.61M ops/s; 0 B, 0 allocs      |
| Membership hit, 14 parallel readers (aggregate) | 5.872–7.158M ops/s; 192 B, 7 allocs | 7.587–7.746M ops/s; 0 B, 0 allocs |

The parallel-reader figure is bounded by `sync.RWMutex.RLock` reader-count
cache-line contention, not by hashing; removing the allocations helps only
modestly there.

### Causes (pprof `-sample_index=alloc_objects` + `go build -gcflags=-m`)

Each probe went through `hashing.HashObject(value, funnel, hashing.Murmur3_128())`
and allocated seven times, one object per site, in equal shares (~14% each):

1. `hashing.Murmur3_128()` was called per probe: its closure (`func literal escapes`)…
2. …and the `hashFunction{}` value boxed into the `HashFunction` interface.
3. `NewHasher` — `&bufferedHasher{}` escapes (passed to an opaque funnel).
4. `bufferedHasher.PutString` — the `append` into a fresh, nil buffer.
5. `NewHashCode` — the defensive copy of the 16-byte digest.
6. `HashCode.Bytes()` — a second defensive copy to read two words.
7. `positions` — a `make([]uint64, k)` slice of probe indexes.

Put on top paid the same seven per value (7 × 1,000 + filter + words = 7002).

### Fixes

- `internal/murmur3.Sum128` is the single MurmurHash3 x64-128 core; `hashing`'s
  `murmur3Sum128` now delegates to it (hashing's public API and outputs are
  unchanged; its Guava vector tests pass). `bloom_filter` calls it directly and
  takes `h1, h2` without building a `HashCode` (removes 1, 2, 5, 6).
- A package-private `probeSink` implements `hashing.PrimitiveSink` with
  byte-identical encodings to hashing's buffered hasher and is recycled through
  a `sync.Pool` (removes 3 and 4 once warm; buffers above 64 KiB are not
  returned to the pool). The sink must escape — the funnel is an opaque
  function value — so pooling, not stack allocation, is the only fix that keeps
  the public `Funnel[T]` signature.
- Put and MightContain iterate the `h1 + i*h2` sequence inline (removes 7).
- `WriteGuavaTo` encodes into one buffer (capped at 6 + 4096×8 bytes) and
  writes it with as few `Write` calls as possible instead of one call per
  8-byte word, so a `bytes.Buffer` destination grows once instead of ~6 times.
- `bloom_filter_ts.Compatible` read both shapes by copying both full bit arrays
  (two O(m) allocations); it now reads `BitCount`/`HashCount` under each
  filter's read lock (O(1), zero allocations, still never two locks at once).

### Harness mismatches fixed on the Go side

- Guava's `bloomFilterMerge` ends with `copy.mightContain(values[0])`; Go now
  does the same (`MightContain("value-0000")`). Under the old hash path that
  added 7 allocations and ~70 ns, which is why "before" merge differs from the
  2026-11-22 row.
- Guava's `bloomFilterSerialization` returns `output.toByteArray()` (a copy);
  Go returned the buffer's internal slice. Go now returns `bytes.Clone`.
- Not fixable on the Go side: every JMH benchmark carries
  `@Setup(Level.Invocation)` to advance `index`. JMH documents that
  invocation-level fixtures add timing overhead per call, which is material for
  nanosecond-scale `mightContain`; Guava's hit/miss figures are therefore
  pessimistic. Moving the index update into the benchmark body would make the
  Java side comparable. Java's index also cycles `& 1023` then `% 1000`, so
  values 0–23 are probed twice per cycle (negligible).

### Rejected experiments

- Per-filter `atomic.Pointer[probeSink]` scratch slot (Swap/Store, pool as
  fallback) instead of the pool on every call: hit 21.1–21.4 ns vs pool
  19.9–20.4 ns (worse), miss 25.4–25.8 vs 26.8–27.2 ns (better), Put better but
  +2 allocs per filter. Mixed and adds a field plus cross-reader cache-line
  traffic in the `_ts` twin; reverted.
- Lemire "fastmod" (128-bit precomputed reciprocal, three `bits.Mul64`) instead
  of `% bitSize`: hit 22.2–22.3 ns vs 19.9–20.4 ns and Put 31.3–32.3 µs vs
  29.8–30.0 µs — slower; Apple M4's hardware divide beats the multiply chain.
  Reverted.
