# iter_util benchmarks: Pluto (Go) vs Direct Go loop

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.

| Workload | Pluto (Go) | Direct Go loop |
|---|---:|---:|
| Concat two 10,000-value sequences | 21.05–21.09K · 152 B, 6 allocs | 46.63–46.82K · 0 B, 0 allocs |
| Filter divisible by three | 137.76–138.75K · 0 B, 0 allocs | 133.64–134.95K · 0 B, 0 allocs |
| Partition into 32-value slices | 83.63–83.96K · 79872 B, 312 allocs | 120.38–121.11K · 80000 B, 313 allocs |
| Pull with idempotent Peek | 2.30–2.30K · 256 B, 7 allocs | 2.77–2.78K · 256 B, 7 allocs |

iter_util has no Java benchmark. Its comparison column is the same work written as a direct Go loop.

## Original record (2026-11-22)

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0. Three 1-second
measurements used the same 10,000 deterministic integers. Corpus construction
is outside the timed loops. Guava comparison is optional for these Go-centric
APIs; direct Go loops and `iter.Pull` are the relevant baselines.

| Workload                          | iter_util                            | Direct Go baseline                 |
|-----------------------------------|-------------------------------------:|-----------------------------------:|
| Concat two 10,000-value sequences | 48.34–49.11 µs; 152 B, 6 allocs      | 21.18–21.48 µs; 0 allocs           |
| Filter divisible by three         | 7.503–7.569 µs; 0 allocs             | 7.676–7.765 µs; 0 allocs           |
| Partition into 32-value slices    | 12.85–13.09 µs; 79,872 B, 312 allocs | 9.09–9.21 µs; 80,000 B, 313 allocs |
| Pull with idempotent Peek         | 445.2–447.0 µs; 256 B, 7 allocs      | 370.1–371.9 µs; 256 B, 7 allocs    |

Peek/Next adds one pull indirection over the baseline's Next-only loop. These
numbers are workload records, not general claims about every iterator shape.
Raw output is `go-matched.txt`; the integrated harness also normalizes these
metrics with the other phases.
