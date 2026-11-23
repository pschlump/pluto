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
