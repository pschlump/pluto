# Phase 5 hashing benchmark record

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
