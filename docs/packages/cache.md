# cache

## Public contract

`Cache[K,V]` and `LoadingCache[K,V]` are concurrent configurable caches. Natural
constructors use comparable keys and maphash; Func forms accept custom key
equality/hash. Options configure maximum size, maximum weight/weigher,
expire-after-write, expire-after-access, refresh-after-write, fake clock, bulk
loader, refresh executor, and synchronous/asynchronous removal listeners.

`GetIfPresent` records access and hit/miss statistics. `Peek` does not change LRU,
access time, or stats. `Put`, `PutIfAbsent`, `Replace`, and `DeleteValue` provide
atomic map-style writes. `Invalidate`, `InvalidateKeys`, `InvalidateAll`, and
`CleanUp` remove entries by explicit cause or policy. `Snapshot`/`All` copy visible
entries and propagate early termination. In-flight placeholders are invisible to
size, expiration, snapshots, Peek, and GetIfPresent.

## Loading and cancellation

`Get` uses the configured loader; `GetWith` accepts one per call. Same-key misses
share one load; unrelated keys load concurrently. No caller loader runs while the
policy lock is held. The initiating context controls the shared load. Other
waiters can cancel independently without stopping it. A same-key recursive Get
using the loader's supplied context returns `ErrRecursiveLoad` instead of waiting
on itself. Loader errors and panics become errors and failed-load statistics.

Refresh-after-write returns the old value immediately while a short-lived
executor task reloads. The refresh detaches caller cancellation. Failed refresh
retains the old value. Direct writes or invalidation remove the loading entry, so
late completion cannot overwrite newer state. Refresh success reports Replaced
and uses a new static weight. No background goroutine exists per cache.

## Shards

The cache is split into independently locked shards (Guava segments). Each
shard owns its hash table, LRU list, write-order list, size/weight budget, and
statistics. `WithConcurrencyLevel(n)` (default 16, `n < 1` panics) targets the
smallest power of two ≥ n. Bounded caches use Guava's rule: doubling stops once
`shards*20` exceeds the smaller of the size and weight bounds, so caches below
20 are a single shard with exact global LRU. The bounds are split exactly
(the remainder goes to the low shards), so the global bound always holds. LRU
order, eviction, and operation-driven expiration are **per shard**, and an
entry heavier than its shard's weight share is evicted immediately even when
the global total has room (as in Guava). The shard index is the top bits of
`hash*0x9E3779B97F4A7C15`, so weak caller hashes still spread.

Single-key writes lock one shard; hits lock nothing (next section). `Len`, `Snapshot`, `Stats`,
`ClearStats`, `InvalidateKeys`, and `InvalidateAll` lock every shard in index
order (the only multi-lock path), so they stay atomic. `CleanUp` visits shards
one at a time. Bulk-load stats have no key and live in shard 0. Expired entries
in shards nobody writes to stay resident until touched, `CleanUp`, or `Len`.

## Lock-free reads

`GetIfPresent`, `Peek`, and `Get` hits take no lock. Each shard has its own
open-addressing table (`table.go`) with atomic slots `{hash, entry}`. Readers
probe it with atomic loads. Writers mutate it under the shard lock. Growth and
tombstone purges publish a whole new array, and the old one is never written
again. Hash 0 means an empty slot and 1 a tombstone (`fixHash` moves key hashes
to ≥ 2). Insert stores the entry and then the hash; delete stores the tombstone
and then a nil entry. Occupancy stays at most half, so every probe ends.

**The table is only a hint; the entry's atomic `state` is the truth.** A reader
returns an entry only when `state == entryLive`. Every other outcome (not
found, loading placeholder, dead, expired-looking, refresh due) retries under
the shard lock, which is exact. A reader on a stale table can therefore never
return an invalidated entry. **Entries are immutable once live.** Key, hash,
value, weight, and writeTime are written before `link` stores `entryLive` (the
release). A refresh does not mutate the value. It publishes a replacement
entry in the same slot (`replaceLocked`) and marks the old one dead.
`accessTime` is atomic. Timestamps are int64 nanoseconds since the cache's
`epoch`.

A hit's LRU touch is buffered in the shard's `readBuffer` (`buffer.go`). This
is a 64-slot multi-producer ring: a producer claims a ticket by CAS, fills the
slot with plain writes, and publishes with an atomic `seq` store. There are no
atomic pointer stores, which cost an out-of-line write-barrier call; profiling
showed that at ~10% of a hit. A reader that leaves ≥ 32 events pending drains
under `TryLock`. Every locked operation drains first (`cleanLocked`,
`presentLocked`, `executeLoad`, `Stats`). **The buffer is lossless, unlike
`tiny_lfu_ts`:** a full ring makes the reader take the lock, drain, and apply
its own event, so hit stats, LRU order, and `Len` stay exact. A claimed but
unpublished slot (a preempted producer) stops the drain at that point. The
full-ring fallback then applies its event directly instead of spinning. Each
drained event is one hit plus one touch.

When nothing reads access order (no size or weight bound and no
expire-after-access, `ordered == false`), reads skip the buffer and only
increment a padded per-shard atomic hit counter. Caffeine disables its read
buffer in the same case.

Access times captured outside the lock can trail the access-list tail by
nanoseconds (an event published just after a concurrent drain). `clampAccess`
raises them to the tail's time, which keeps the access list sorted for the
expiry drain and extends such an entry's life by at most that window. The
lock-free expiry check sees only applied access times, so a buffered access
can make a live entry look expired. The locked retry drains first and gets it
right (`TestBufferedAccessExtendsExpiry`).

Mutation-tested guards, each of which fails when its rule is removed: the state
check (`TestInFlightPlaceholderInvisibleToLockFreeReads`,
`TestStaleTableDeadEntryNotReturned`), the drain-before-recheck rule (expiry
tests), and losslessness (`TestFullReadBufferIsLossless`).
`TestLockFreeReadersDuringGrowthAndChurn` races readers against growth,
tombstone churn, and replacement.

## Expiration, eviction, and notifications

Expiration uses `now.Sub(timestamp) >= duration`, with exact fake-clock boundary
tests. Access reads update only GetIfPresent/Get, not Peek. Maximum size and weight
use an LRU list and static write-time weight. Negative weights and weight overflow
are programmer errors. Maximum zero discards values immediately. Expired and size
removals both increment `Evictions`, matching Guava; explicit invalidation and
replacement do not.

Expiration drains from the front of time-ordered lists: O(expired) per call.
**Never reintroduce a whole-table scan.** The former per-write O(n) scan, which
ran even with expiration off, made writes ~50× slower than Guava (see
docs/benchmarks/cache/README.md). The access list (the LRU list) is ordered by
access time. The write list (`wprev`/`wnext`, maintained only with
expire-after-write) is ordered by write time. That ordering holds only because
**write times are taken under the shard lock and access times are clamped**
(`clampAccess`, previous section). That is why `executeLoad` reads `finished`
after locking. A refresh completion is a write, so `replaceLocked` links the
replacement entry at the tail of both lists (Guava's `recordWrite`). With
expiration off the clock is never read on hits or writes (`stamp`), and access
times are recorded only with expire-after-access. Write operations, including
`Invalidate`, run the shard's expiry drain first, so invalidating an
already-expired entry reports `Expired`, as in Guava. The drain panics with an
internal-error message if a linked entry is missing from its table (an
invariant violation that would otherwise loop forever).
`TestExpirationQueueModel` checks the queues against a brute-force
every-key-every-step reference at 1 and 16 shards. A mutation that skips the
LRU move makes it fail.

Notifications identify Explicit, Replaced, Expired, Size, and LoaderError. They
run after internal locks are released, so listeners may re-enter the cache.
Synchronous delivery is the default; a listener executor opts into asynchronous
dispatch.

## Bulk loads and stats

`GetAll` preserves key order. Without a bulk loader it performs coalesced per-key
loads. A bulk loader receives distinct currently missing keys. Partial entries are
retained and returned with a non-nil error; missing, duplicate, or extra keys
produce `ErrBulkLoad`. Stored zero values remain present. Bulk loads do not
coalesce with concurrent per-key loads.

`Stats` includes hits, misses, actual loads, successes, failures, total load time,
and evictions. `ClearStats` resets the snapshot counters.

## Verification and comparison

Tests use fake clocks and channels rather than correctness sleeps. They cover all
expiration boundaries, access versus write timestamps, size/weight eviction,
coalescing, unrelated-key concurrency, cancellation, recursion, panic/error,
refresh success/failure/direct-write races, bulk partial success, every removal
cause, listener re-entry, atomic operations, and race stress. The pinned Guava
oracle compares loading/expiration traces and stats. Matched Go/JMH benchmarks
cover mixed hits, evicting writes, forced/coalesced misses, expiration, refresh,
8/16-thread hits, and Zipf, scan, and phase-change inputs, with Pluto
LRU/TinyLFU baselines and results in `docs/benchmarks/cache/`.
