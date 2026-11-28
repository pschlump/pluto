# bimap benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| HashBiMap mixed stream | 29.87–30.68M · 0 B, 0 allocs | 20.24M ± 2.30M |
| Inverse iteration, 4096 entries | 144.93–145.94K · 0 B, 0 allocs | 28.87K ± 6.25K |
| Baseline: two plain maps, mixed stream | 40.78–41.32M · 0 B, 0 allocs | 58.50M ± 8.93M |

Retained memory: about 192 bytes per association for Pluto HashBiMap and for two plain Go maps alike. Go and Java heap figures are not directly comparable.

## Original record (2026-11-22)

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

## 2026-11-27: hash_grow probe fixes

Pluto's bimap already beat Guava on both workloads. The gap that mattered was
the two-plain-maps baseline. A profile showed two costs in `hash_grow`, which
backs both directions:

1. **Equality on every probed slot.** `Search`, `Delete` and insert stored
   each slot's hash but never compared it, so they called the equality
   closure (two indirect calls plus a string compare) on every occupied slot
   along the chain. They now check the stored hash first. This is safe
   because equal elements must hash equal.
2. **An integer division per probe** (`rh % size`). Table sizes are powers of
   two whenever the constructor's `n` is (all in-repo callers), and growth
   only doubles, so the home bucket is now a mask, with `%` kept for other
   sizes.

bimap's own code is unchanged. Same machine and corpus; raw output is
`go-hashgrow.txt`. The "before" column was re-measured on the same machine
state from the prior commit.

| Workload                          | Before                 | After                  | Guava (JMH)       |
|-----------------------------------|-----------------------:|-----------------------:|------------------:|
| HashBiMap mixed stream            | 24.7–25.4M ops/s       | 29.95–31.29M ops/s     | 20.239M ± 2.300M  |
| Two Go maps (baseline)            | 40.5–41.7M             | 40.5–41.7M (unchanged) | Java: 58.503M     |
| 4096-entry inverse iteration      | 142–146K               | 145.7–147.3K           | 28,872 ± 6,249    |

Memory is unchanged (about 192 bytes/association). Mixed operations are about
1.5× Guava's rate and about 75% of the raw two-map baseline.

Two experiments were measured and **rejected**:

- **Hash each argument once per operation** (store both hashes in the table
  element and pass them in probes). Once `hash_grow` stopped calling equality
  on mismatched slots, this gained nothing (33.5–34.3 ns vs 32.5–33.7 ns for
  the unchanged code). It grew the element from 32 to 40–48 bytes, which made
  inverse iteration about 50% slower (6.9 → 10.5 µs) and raised memory to
  about 224 bytes/association.
- **Hash stored inline in `hash_grow` slots** (one array of `{hash, item}`
  instead of two parallel arrays). bimap gained about 3%, but `hash_set`'s
  mixed workload lost about 4%, because wider slots make long probe runs
  touch more cache lines.

The remaining gap to Go's built-in map is `hash_grow`'s design. It uses
linear probing at load ≤ 0.5 with separate hash and element arrays, while Go's
Swiss-table map keeps control bytes next to each slot group.
