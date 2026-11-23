# range_map / range_map_ts

`RangeMap[T,V]` associates disjoint ordered ranges with values. Put removes all
overlapping mappings, preserves non-overlapping fragments, and inserts the new
entry. Remove subtracts exactly. Adjacent entries are not coalesced even when
values are equal. Empty ranges are no-ops. Optional discrete domains canonicalize
inputs and fragments.

Sorted-slice Put/Remove are O(r), point lookup is O(r), and ordered iteration is
O(r), where r is the entry count. `Get` distinguishes absence from a zero value;
`RangeContaining` returns the stored range too. `SubRangeMap`, `Clone`, `Entries`,
and `EntriesSlice` are snapshots. Equality takes a value function and therefore
supports non-comparable V.

`range_map_ts` adds atomic writes and snapshot reads under `sync.RWMutex`.
`immutable.FreezeRangeMap` creates an immutable snapshot with point/range queries,
subranges, equality, and safe concurrent reads. The pinned Guava trace verifies
split boundaries and replacement behavior.
