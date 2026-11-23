# range_set / range_set_ts

`RangeSet[T]` stores ordered disjoint ranges. Natural and custom-comparator
constructors are joined by discrete-domain forms. Empty Add/Remove is a no-op.
Add coalesces connected ranges; discrete construction canonicalizes first and
also merges successor-adjacent ranges. Remove preserves exact open/closed left
and right fragments.

The implementation uses a sorted slice: Add/Remove are O(r), point containment
and range-containing queries are O(r), and iteration is O(r), where r is the
number of stored ranges. This favors compact storage and straightforward exact
boundary behavior; benchmarks compare against Guava's tree implementation.

`Complement` covers the full comparator universe, including unbounded ends.
`SubRangeSet`, `Complement`, `Clone`, `Slice`, and `All` return independent
snapshots. `Encloses`, `Span`, `Len`, and `Equal` complete the read contract.

`range_set_ts` provides atomic mutation and snapshot reads with `sync.RWMutex`.
Opposite-way equality never holds two locks. `immutable.FreezeRangeSet` creates
a read-only snapshot retaining discrete-domain behavior; it supports queries,
complement, subranges, and concurrent reads without a twin.
