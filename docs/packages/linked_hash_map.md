# linked_hash_map / linked_hash_map_ts

`Map[K,V]` is a hash map with deterministic first-insertion iteration. Updating
a value keeps its position. Deleting and later reinserting a key appends it.
Natural construction uses `==` and `maphash`; `NewFunc` accepts matching
key-equality and hash functions. Stored keys retain the first inserted key value,
as Go map assignment does.

| Operation | Complexity |
|---|---|
| Get / ContainsKey / Put / Delete | expected O(1) |
| Len / IsEmpty | O(1) |
| Entries / Keys / Values | O(n), insertion order |
| Clear / Clone | O(n) |

`Put` returns the previous value and presence flag. `Delete` returns the removed
value and presence flag. Iterators expose no mutable internal slices. The plain
map must not be mutated during iteration or used concurrently.

The `_ts` twin protects compound operations with one RWMutex. Iterator methods
materialize snapshots under a read lock and release it before caller code runs.
No twin method holds locks belonging to two maps.

JSON uses an insertion-ordered array of `{key,value}` entries, avoiding Go's
restrictions on generic JSON object keys. Decoding validates before replacement
and duplicate keys update their first position.

Tests include a fixed-seed built-in-map model, collisions, custom equality,
update/delete/reinsert order, cloning, JSON failure atomicity, early stop, twin
snapshots, and concurrent access. `Phase9Oracle` generates
`linked_hash_map/testdata/guava_linked_map.json` from `LinkedHashMap`. Matched
Go/JMH put/get and iteration benchmarks are recorded under `docs/benchmarks/phase9/`.
