# bloom_filter and bloom_filter_ts

`BloomFilter[T]` uses `hashing.Funnel[T]` and Guava's MURMUR128_MITZ_64 strategy.
The lower and upper little-endian 64-bit halves form `h1+i*h2`; each index is
`(combined & MaxInt64) % bitSize`. Guava rounds bitSize to the allocated word
count, so Pluto does too. This detail is required for bit-array compatibility.

New computes optimal storage/hash count and clamps hashes to [1,255].
Put/MightContain are O(encoded bytes + k). Copy is O(m/64). Merge is O(k*m/64),
validates all operands before mutation, and maintains an exact set-bit counter.
ExpectedFPP is saturation^k; Count is the standard fill-ratio estimate. Full
saturation returns 1 and bitSize respectively. Compatible checks shape/strategy;
function values cannot prove semantic funnel equality, so callers must use the
same funnel contract when merging or decoding.

Pluto PBF1 encoding is magic/version + little-endian bit size/hash count/words.
Guava compact encoding is explicitly available through WriteGuavaTo,
ReadGuavaFrom, and FromGuavaBytes (strategy byte 1, hash count, big-endian word
count/words). Guava decoding rounds to whole words. Neither codec accepts the
legacy bloom.Bytes representation.

The `_ts` twin takes an RWMutex around each operation. Merge snapshots operands
under one read lock at a time, then validates all snapshots under the destination
write lock before mutation; opposite merges cannot deadlock. Funnels run under
the calling filter's lock and must not re-enter that same filter.

Golden differential tests compare every word and Guava stream byte for 1000 UTF-8
strings. Other tests cover disjoint-set FPP, approximate count, merge atomicity,
version/length corruption, stream round trips, concurrent writes/reads/copies,
and opposite merges under -race. Benchmarks are in docs/benchmarks/bloom_filter/.

## Allocation-free probe path (2026-11-27) — invariants to keep

Put/MightContain allocate nothing once warm (pinned by `TestProbeAllocations`
in both packages; skipped under `-race`, where `sync.Pool` drops items on
purpose). The path is `hash128`: pooled `probeSink` → funnel →
`internal/murmur3.Sum128(sink.data, 0)` → inline `h1 + i*h2` loop.

- `probeSink` must encode every `PrimitiveSink` method byte-for-byte like
  `hashing`'s buffered hasher (little-endian integers, IEEE bits, raw UTF-8).
  If hashing ever changes a primitive's encoding, change the sink with it.
  `TestFastHashMatchesHashObject` compares both paths over 5,000 fixed-seed
  values for string, bytes, int64 and an all-12-methods record funnel.
- `hash1` is `Sum128`'s first return (the low little-endian word of
  `HashCode.Bytes()`), `hash2` the second; seed 0. Swapping them breaks the
  Guava golden immediately.
- `getProbeSink` must truncate the buffer (`data[:0]`); the pool hands back
  dirty sinks. Buffers above 64 KiB are dropped rather than pooled.
- Never hash through `hashing.HashObject`/`Murmur3_128()` on the hot path —
  that path costs 7 allocations per probe. `referenceHashes`/`referencePut`
  in `fast_path_test.go` keep the old derivation as the oracle.
- Funnels must not retain the sink. Re-entrant funnels (calling back into the
  same filter) are safe: each call takes its own pooled sink
  (`TestReentrantFunnel`).
- `WriteGuavaTo` writes one encode buffer of at most `guavaWriteChunk` bytes
  per call; output must stay byte-identical to the word-at-a-time encoding
  (`TestWriteGuavaChunking` crosses the chunk boundary and partial writers).
- `bloom_filter_ts.Compatible` reads each shape under its own read lock; do not
  reintroduce full-filter copies there. Nil and zero-value filters are
  compatible only with each other (`TestCompatibleShapes`).

Rejected (measured slower or mixed): a per-filter atomic scratch sink, and
Lemire fastmod for `% bitSize` — see docs/benchmarks/bloom_filter/README.md.
