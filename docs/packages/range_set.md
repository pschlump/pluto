# range_set / range_set_ts

`RangeSet[T]` stores ordered disjoint ranges. Natural and custom-comparator
constructors are joined by discrete-domain forms. Empty Add/Remove is a no-op.
Add coalesces connected ranges; discrete construction canonicalizes first and
also merges successor-adjacent ranges. Remove preserves exact open/closed left
and right fragments.

The implementation uses a sorted slice (r = number of stored ranges).
Add/Remove binary-search the contiguous run of stored ranges connected to the
argument (`connectedWindow`) and splice it in place with `slices.Replace`:
O(log r) comparisons plus an O(r) worst-case memmove, no per-call allocation.
Contains/RangeContaining/Encloses are O(log r); iteration is O(r).

**The invariant the binary searches depend on:** stored ranges are sorted,
pairwise disconnected, and never empty. Disjointness makes upper bounds ascend
with lower bounds, which makes both halves of `IsConnected` (`lowerAfterUpper`)
monotone over the slice. A stored empty range such as `(a,a)` breaks the
monotonicity and silently corrupts lookups — so every fragment `subtract`
produces is checked with `IsEmpty` before it is stored. (Before 2026-11-27 the
whole-slice code stored one: removing up to a finite domain's maximum from an
unbounded range left the canonical remainder `(max,max)`.) The successor
adjacency loops in Add are unreachable for canonical ranges (Canonical opens
every closed upper bound that has a successor) but are kept so Add matches the
`adjacent` predicate exactly. `reference_test.go` holds the original
whole-slice algorithms as differential oracles — keep it in step with any
semantic change.

`Complement` covers the full comparator universe, including unbounded ends.
`SubRangeSet`, `Complement`, `Clone`, `Slice`, and `All` return independent
snapshots. `Encloses`, `Span`, `Len`, and `Equal` complete the read contract.

`range_set_ts` provides atomic mutation and snapshot reads with `sync.RWMutex`.
Opposite-way equality never holds two locks. `immutable.FreezeRangeSet` creates
a read-only snapshot retaining discrete-domain behavior; it supports queries,
complement, subranges, and concurrent reads without a twin.
