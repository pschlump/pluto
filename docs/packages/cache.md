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

## Expiration, eviction, and notifications

Expiration uses `now.Sub(timestamp) >= duration`, with exact fake-clock boundary
tests. Access reads update only GetIfPresent/Get, not Peek. Maximum size and weight
use an LRU list and static write-time weight. Negative weights and weight overflow
are programmer errors. Maximum zero discards values immediately. Expired and size
removals both increment `Evictions`, matching Guava; explicit invalidation and
replacement do not.

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
