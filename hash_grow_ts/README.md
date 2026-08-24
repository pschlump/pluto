# hash_grow_ts

`hash_grow_ts` is a generic, **thread-safe** hash table for Go using open
addressing with linear probing. When the load factor exceeds a configurable
saturation threshold (default 0.5) the table automatically doubles in size
and re-hashes all entries. Every operation is guarded by a `sync.RWMutex`.

It provides an API identical to [`hash_grow`](../hash_grow); see that package
for the non-locking variant.

Elements must implement `comparable.Comparable` (a `Compare` method). Hash
keys come from, in priority order:

1. the `Hashable` interface (`HashKey(x any) int`), if the element implements it,
2. the element itself if it is a `string`,
3. `fmt.Stringer`, hashed with FNV-1a.

## Complexity

| Operation                                | Average | Worst case | Notes                                    |
|------------------------------------------|---------|------------|------------------------------------------|
| `Insert`                                 | O(1)    | O(n)       | amortized; growth doubles the table      |
| `Search`                                 | O(1)    | O(n)       | linear probe from the home bucket        |
| `Delete`                                 | O(1)    | O(n)       | backward-shift keeps probe chains intact |
| `IsEmpty`, `Len`, `Length`               | O(1)    | O(1)       |                                          |
| `Truncate`                               | O(n)    | O(n)       | clears all buckets for GC                |
| `Walk`, `Print`, `Dump`, `All`, `Values` | O(n)    | O(n)       |                                          |

Worst case occurs when many keys collide; the load factor is kept below the
saturation threshold (default 0.5) so average behavior is O(1).

## Usage

```go
ht := hash_grow_ts.NewHashTab[Item](16, 0) // initial size 16, default saturation 0.5

ht.Insert(&Item{Key: "alpha"})

if it := ht.Search(&Item{Key: "alpha"}); it != nil {
	fmt.Println("found", it.Key)
}

ht.Delete(&Item{Key: "alpha"})

// Range-over-func iteration (Go 1.23+), safe to call table methods in the body:
for pos, item := range ht.All() {
	fmt.Printf("bucket %d: %s\n", pos, item.Key)
}
```

## Iteration

Two styles are supported:

- Go 1.23+ range-over-func: `All() iter.Seq2[int, *T]` (bucket position and
  element) and `Values() iter.Seq[*T]`. Both iterate over a **snapshot** of
  the table taken under the read lock when iteration begins, so it is safe
  (and deadlock-free) to call other table methods from the loop body.
- The legacy `Walk(ApplyFunction[T], userData)` callback API. `Walk` holds
  the read lock for its entire duration, so the callback must **not** call
  other methods of the table.

## Thread safety

All exported methods are safe for concurrent use. `Search` and other read
operations take the read lock; `Insert`, `Delete` and `Truncate` take the
write lock. Note that a `*T` returned by `Search` (or yielded by an
iterator) is shared with the table — treat it as read-only unless you
synchronize access to the element yourself.
