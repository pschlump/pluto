# hash_tab_bt_ts

Package `hash_tab_bt_ts` — a generic, **thread-safe** hash table for Go
where each bucket is a binary search tree
(`github.com/pschlump/pluto/binary_tree_ts`).  Because collisions are
resolved by a tree rather than a linked list, lookups stay logarithmic even
under heavy collisions.

This is the thread-safe variant of
[`hash_tab_bt`](../hash_tab_bt); both packages expose an identical API.

## Overview

- Generic over any `T` implementing `comparable.Comparable`.
- Items are stored as `*T`; inserting an item equal to an existing one
  replaces it.
- The hash key is derived from the item itself: if `T` (or `*T`) implements
  `Hashable` (`HashKey(x any) int`) that is used, otherwise
  `fmt.Stringer`.  Items implementing neither cause a panic.
- The number of buckets is fixed at construction (`NewHashTab(n)`, `n >= 5`).

## Complexity

| Operation                   | Average     | Notes                 |
|-----------------------------|-------------|-----------------------|
| `Insert`                    | O(log(n/k)) | k = number of buckets |
| `Search`                    | O(log(n/k)) |                       |
| `ItemExists`                | O(log(n/k)) |                       |
| `Delete`                    | O(log(n/k)) |                       |
| `IsEmpty`                   | O(1)        |                       |
| `Len` / `Length`            | O(1)        |                       |
| `Truncate`                  | O(k)        | resets every bucket   |
| `Walk` / `WalkFunc` / `All` | O(n)        | visit every element   |
| `Dump`                      | O(n)        |                       |

Worst case (all keys in one bucket) degenerates to the binary tree's
O(n) per operation.

## Example

```go
package main

import (
	"fmt"

	hash_tab "github.com/pschlump/pluto/hash_tab_bt_ts"
)

func main() {
	ht := hash_tab.NewHashTab[Item](97) // Item implements comparable.Comparable + Hashable
	ht.Insert(&Item{Key: "alpha"})

	if ht.ItemExists(&Item{Key: "alpha"}) {
		fmt.Println("found")
	}

	// Go 1.23+ range-over-func iteration over a consistent snapshot:
	for item := range ht.All() {
		fmt.Println(item.Key)
	}

	ht.Delete(&Item{Key: "alpha"})
}
```

## Iteration

Three styles are supported, all in bucket order (not sorted order):

- `All()` — modern Go 1.23+ range-over-func iterator (`iter.Seq[*T]`).
  A consistent snapshot of the table is taken under a read lock when `All`
  is called, so it is safe to call other table methods from inside the
  loop.
- `WalkFunc(fx func(*T))` and `Walk(fx, userData)` — callbacks invoked
  while the read lock is held; the callback must not call back into the
  table or it will deadlock.

## Thread safety

All exported methods are guarded by a `sync.RWMutex`; the zero-value rules
are the usual ones — do not copy a `HashTab` after first use.

For batching several operations atomically, hold the lock explicitly and
use the no-lock (`Nl`-prefixed) methods:

```go
ht.ReadLock()
item := ht.NlSearch(&key)
ht.ReadUnlock()

ht.WriteLock()
ok := ht.NlDelete(&key)
ht.WriteUnlock()
```
