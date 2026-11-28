# hashing benchmarks: Pluto (Go) vs Guava (Java)

## Current results

Rates are operations per second; higher is better. The Go column was
measured 2026-11-27 on an Apple M4 Max, darwin/arm64, Go 1.27.0: three
1-second runs, shown slowest–fastest, plus bytes and allocations per
operation. Raw output is `go-2026-11-27-current.txt`.
The Java column is Guava under JMH on OpenJDK 11.0.28, copied from the
2026-11-22 record below and not re-run (mean ± error).

| Workload | Pluto (Go) | Guava (Java) |
|---|---:|---:|
| Murmur3-128, 16 B | 79.43–80.45M · 16 B, 1 alloc | 81.31M ± 13.99M |
| Murmur3-128, 64 B | 63.86–64.35M · 16 B, 1 alloc | 23.68M ± 0.82M |
| Murmur3-128, 1024 B | 8.78–8.79M · 16 B, 1 alloc | 3.59M ± 2.48M |
| Murmur3-32, 1024 B | 3.22–3.23M · 4 B, 1 alloc | 1.47M (high variance) |
| SipHash-2-4, 64 B | 37.05–37.06M · 8 B, 1 alloc | 8.42M (high variance) |
| FarmHashFingerprint64, 1024 B | 16.85–16.91M · 8 B, 1 alloc | 18.46M ± 0.55M |
| Fingerprint2011, 1024 B | 17.50–17.53M · 8 B, 1 alloc | 17.35M ± 0.91M |
| Murmur3-128 stream, 1024 B in 1 B chunks | 237.08–239.35K · 3384 B, 11 allocs | 0.56M ± 0.40M |
| Murmur3-128 stream, 1024 B in 7 B chunks | 1.01–1.01M · 3384 B, 11 allocs | 0.84M (high variance) |
| Murmur3-128 stream, 1024 B in 64 B chunks | 2.21–2.22M · 3328 B, 8 allocs | 1.97M ± 0.55M |
| Jump consistent hash, 1024 buckets | 53.22–53.28M · 0 B, 0 allocs | 44.88M ± 2.96M |

## Original record (2026-11-22)

Recorded 2026-11-22 on Apple M4 Max, darwin/arm64, Go 1.27.0, OpenJDK 11.0.28,
Guava commit `36e3995499d6c3b74b1207d362fa9e19915d2589`. Inputs are deterministic
byte sequences. Go ran three 1-second measurements; JMH 1.37 ran two 1-second
warmups and three 1-second measurements with one fork and one thread.

All comparison rates use operations/second (K=thousand, M=million,
G=billion). Go ranges are reciprocal conversions of measured latency,
slowest to fastest; JMH rates are reported directly.

| Workload                                 | Go rate / allocations                     | Guava JMH rate               |
|------------------------------------------|------------------------------------------:|-----------------------------:|
| Murmur3-128, 16 B                        | 76.046–77.220M ops/s; 16 B, 1 alloc       | 81.305M ops/s ± 13.985M      |
| Murmur3-128, 64 B                        | 60.423–61.576M ops/s; 16 B, 1 alloc       | 23.681M ops/s ± 0.822M       |
| Murmur3-128, 1024 B                      | 8.606–8.621M ops/s; 16 B, 1 alloc         | 3.588M ops/s ± 2.479M        |
| Murmur3-32, 1024 B                       | 3.194–3.204M ops/s; 4 B, 1 alloc          | 1.467M ops/s (high variance) |
| SipHash-2-4, 64 B                        | 36.101–36.337M ops/s; 8 B, 1 alloc        | 8.416M ops/s (high variance) |
| FarmHashFingerprint64, 1024 B            | 16.466–16.548M ops/s; 8 B, 1 alloc        | 18.457M ops/s ± 0.549M       |
| Fingerprint2011, 1024 B                  | 17.071–17.153M ops/s; 8 B, 1 alloc        | 17.349M ops/s ± 0.906M       |
| Murmur3-128 stream, 1024 B / 1 B chunks  | 229.885–232.558K ops/s; 3384 B, 11 allocs | 0.558M ops/s ± 0.398M        |
| Murmur3-128 stream, 1024 B / 7 B chunks  | 958.773–964.320K ops/s; 3384 B, 11 allocs | 0.839M ops/s (high variance) |
| Murmur3-128 stream, 1024 B / 64 B chunks | 2.073–2.116M ops/s; 3328 B, 8 allocs      | 1.968M ops/s ± 0.554M        |
| Jump consistent hash, 1024 buckets       | 52.659–52.854M ops/s; 0 allocs            | 44.875M ops/s ± 2.955M       |

The current general Go Hasher buffers input; direct HashBytes is substantially
cheaper for these workloads. Java and Go measurements use different runtimes and
metrics; no speed ratio is inferred. Consistent-hash correctness separately checks
minimal remapping for 10,000 keys and a 50–150 assignments-per-bucket distribution
band over 100 buckets.

Raw files: `go-matched.txt`, `jmh-matched.json`; normalized metrics are in
`normalized.json`. Reproduce with `make guava-bench`.
