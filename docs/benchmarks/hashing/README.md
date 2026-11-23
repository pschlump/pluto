# Phase 5 hashing benchmark record

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`. Inputs are deterministic
byte sequences. Go ran three 1-second measurements; JMH 1.37 ran two 1-second
warmups and three 1-second measurements with one fork and one thread.

| Workload | Go latency / allocations | Guava JMH throughput |
|---|---:|---:|
| Murmur3-128, 16 B | 12.95–13.15 ns; 16 B, 1 alloc | 81.305M ops/s ± 13.985M |
| Murmur3-128, 64 B | 16.24–16.55 ns; 16 B, 1 alloc | 23.681M ops/s ± 0.822M |
| Murmur3-128, 1024 B | 116.0–116.2 ns; 16 B, 1 alloc | 3.588M ops/s ± 2.479M |
| Murmur3-32, 1024 B | 312.1–313.1 ns; 4 B, 1 alloc | 1.467M ops/s (high variance) |
| SipHash-2-4, 64 B | 27.52–27.70 ns; 8 B, 1 alloc | 8.416M ops/s (high variance) |
| FarmHashFingerprint64, 1024 B | 60.43–60.73 ns; 8 B, 1 alloc | 18.457M ops/s ± 0.549M |
| Fingerprint2011, 1024 B | 58.30–58.58 ns; 8 B, 1 alloc | 17.349M ops/s ± 0.906M |
| Murmur3-128 stream, 1024 B / 1 B chunks | 4.30–4.35 µs; 3384 B, 11 allocs | 0.558M ops/s ± 0.398M |
| Murmur3-128 stream, 1024 B / 7 B chunks | 1.037–1.043 µs; 3384 B, 11 allocs | 0.839M ops/s (high variance) |
| Murmur3-128 stream, 1024 B / 64 B chunks | 472.7–482.4 ns; 3328 B, 8 allocs | 1.968M ops/s ± 0.554M |
| Jump consistent hash, 1024 buckets | 18.92–18.99 ns; 0 allocs | 44.875M ops/s ± 2.955M |

The current general Go Hasher buffers input; direct HashBytes is substantially
cheaper for these workloads. Java and Go measurements use different runtimes and
metrics; no speed ratio is inferred. Consistent-hash correctness separately checks
minimal remapping for 10,000 keys and a 50–150 assignments-per-bucket distribution
band over 100 buckets.

Raw files: `go-matched.txt`, `jmh-matched.json`; normalized metrics are in
`normalized.json`. Reproduce with `make guava-bench`.
