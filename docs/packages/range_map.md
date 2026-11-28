# range_map / range_map_ts

`RangeMap[T,V]` associates disjoint ordered ranges with values. Put removes all
overlapping mappings, preserves non-overlapping fragments, and inserts the new
entry. Remove subtracts exactly. Adjacent entries are not coalesced even when
values are equal. Empty ranges are no-ops. Optional discrete domains canonicalize
inputs and fragments.

Storage is a sorted slice (r = entry count). Put/Remove binary-search the
contiguous run of entries connected to the argument (`connectedWindow`),
rebuild only that run in a fixed stack buffer, and splice it in place: O(log r)
comparisons plus an O(r) worst-case memmove, no per-call allocation. Put's
new entry is placed by one insertion step (the rest of the run is already
ordered), replacing the former whole-slice `sort.SliceStable`. Get and
RangeContaining are O(log r); ordered iteration is O(r).

**The invariant the binary searches depend on:** entries are sorted, pairwise
disjoint (touching is allowed — entries are not coalesced), and never empty, so
upper bounds ascend with lower bounds and both halves of `IsConnected` are
monotone. A stored empty range breaks that and silently corrupts lookups;
`subtract` drops fragments that canonicalize to empty. (Before 2026-11-27 the
whole-slice code stored one: overwriting up to a finite domain's maximum inside
an unbounded entry left `(max,max)`.) `reference_test.go` keeps the original
whole-slice algorithms as differential oracles. `Get` distinguishes absence from a zero value;
`RangeContaining` returns the stored range too. `SubRangeMap`, `Clone`, `Entries`,
and `EntriesSlice` are snapshots. Equality takes a value function and therefore
supports non-comparable V.

`range_map_ts` adds atomic writes and snapshot reads under `sync.RWMutex`.
`immutable.FreezeRangeMap` creates an immutable snapshot with point/range queries,
subranges, equality, and safe concurrent reads. The pinned Guava trace verifies
split boundaries and replacement behavior.
