# lru_disk_ts notes

`LruDisk[V]` is the two-tier, persistence-backed LRU: an optional
in-memory tier (a `*lru.Lru[string, V]` holding decoded values) in
front of the disk tier (`*lru.Lru[string, entry]` — one index record
per live file, `entry` is just the path), behind one `sync.RWMutex`.
No plain twin (the `b_tree_disk_ts` precedent — a disk store is a `_ts`
beast by nature). Built for glasm's on-disk script-artifact cache
(`wazero-performance-001`).

**Two bounds, both in entries.** Disk: the constructor's `capacity`
(evict = delete the file; data lost). Memory: `WithMemCapacity(n)` as
a variadic `Option` on `NewLruDisk` (quicklist's Option pattern — no
type parameter needed, the only knob is an int). `n <= 0` (also the
default, no options) leaves the tier off: `c.mem == nil`, every `Get`
reads + decodes the file, nothing retained — the historic pure-disk
contract, so the pre-tier call signature and behavior are unchanged.
`lru.NewLru` panics below 1, hence nil-means-off rather than a 0-cap
inner Lru.

**The load-bearing invariants.**

- `mem ⊆ inner` — every resident value has a live file. Held because
  only `Get`-from-disk, `Put` (both tiers), and `reindex` (inner only)
  insert, while `Delete`/`Clear`/disk-eviction remove from both.
- `inner`'s order is the TRUE global recency: a memory-tier hit still
  calls `inner.Get(key)` (bump only) so a disk eviction always picks
  the globally coldest entry, not the coldest disk-only one.
- `mem` holds the last ≤memCapacity distinct keys *accessed* (every
  access inserts), which is why the inner-LRU victim is resident only
  when the tiers overlap fully (memCap ≥ diskCap) — `nlPut` still
  deletes the victim from `mem` unconditionally (nil- and
  absent-tolerant, self-proofing against order drift).
- Demotion (mem full on promote) removes only the resident value — the
  file stays. THE semantic difference from eviction; a demoted key is
  a slower hit, never a miss.

**Traps.**

- `nlPopOldest` decodes the file BEFORE `os.Remove` — the original
  code removed first and could never decode (the file was gone), so a
  post-restart `PopOldest` silently returned zero V with ok=true.
  `TestTieredPopOldestDecodesFromDisk` is the regression guard.
- `Put` on a zero-value/nil `LruDisk` returns an error (not a panic —
  an I/O-flavored package errors where it can); `Put` on an inner-but-
  no-mem cache must skip `mem.Put` (panics on nil) — the nil-tolerant
  `lru` methods (Get/Peek/Delete/Len/Capacity/Clear) are called bare,
  `Put` never is.
- Self-heal (unreadable/undecodable ⇒ miss + file removed) runs on the
  disk-read path only; a resident value is served without touching the
  file — a torn file under a resident key heals on demote, not on Get.
- A failed `writeEntry` still updates both tiers: tier off ⇒ next Get
  self-heals to a miss; tier on ⇒ the resident value is served until
  it demotes (documented, not "fixed").
- Reindex recency is file mtime (best-effort); the mem tier starts
  empty after a restart and refills lazily — no pre-warm.
- First-error-wins in `nlPut`: the eviction remove error is returned
  even when the payload write succeeded. Eviction fires only for a NEW
  key at capacity — an overwrite replaces in place and never evicts
  (the `lru.Put` contract; the original code "made room" for keys
  that needed none, `TestOverwriteAtCapacity` is the guard).

**Benchmarks** (`bench_test.go`): `BenchmarkGetZipf300K` — 50 keys ×
300 KB incompressible values, Zipf(1.1) stream (top-20 keys ≈ 83.4% of
mass, fixed seed), mem = 0/20/50. M4 Max measured ~33 µs (pure disk,
OS page cache — not cold storage), ~7.85 µs (hot-set tier, ~4.2×), ~86
ns (all-resident — the tier's own overhead, zero allocs; the disk path
allocates the full 300 KB per read). Numbers and the honest page-cache
caveat live in `lru_disk_ts/README.adoc`; identity codec — a real
codec adds cost to every disk read, amortized by the tier to once per
promotion.

Tests: stdlib-only; `newTest`/`newTestMem` helpers, `countLdc` counts
`.ldc` files. Mtime-order tests sleep 2 ms between puts (reindex ties
are arbitrary by contract). Run `-race` (the concurrent tests hammer
both tiers).
