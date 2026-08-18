# hash_grow

`hash_grow` is a generic hash table for Go using open addressing with linear
probing. When the load factor exceeds a configurable saturation threshold
(default 0.5) the table automatically doubles in size and re-hashes all
entries.

Elements must implement `comparable.Comparable` (a `Compare` method). Hash
keys come from, in priority order:

1. the `Hashable` interface (`HashKey(x any) int`), if the element implements it,
2. the element itself if it is a `string`,
3. `fmt.Stringer`, hashed with FNV-1a.

This is the **non-locking** variant. For concurrent use see
[`hash_grow_ts`](../hash_grow_ts), which provides an identical API guarded by
a `sync.RWMutex`.

## Complexity

| Operation  | Average  | Worst case | Notes                                  |
|------------|----------|------------|----------------------------------------|
| `Insert`   | O(1)     | O(n)       | amortized; growth doubles the table    |
| `Search`   | O(1)     | O(n)       | linear probe from the home bucket      |
| `Delete`   | O(1)     | O(n)       | backward-shift keeps probe chains intact |
| `IsEmpty`, `Len`, `Length` | O(1) | O(1) |                            |
| `Truncate` | O(n)     | O(n)       | clears all buckets for GC              |
| `Walk`, `Print`, `Dump`, `All`, `Values` | O(n) | O(n) |              |

Worst case occurs when many keys collide; the load factor is kept below the
saturation threshold (default 0.5) so average behavior is O(1).

## Usage

```go
package main

import (
	"fmt"
	"os"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/hash_grow"
)

type Item struct {
	Key string
}

func (a Item) Compare(x comparable.Comparable) int {
	b := x.(Item)
	if a.Key < b.Key {
		return -1
	} else if a.Key > b.Key {
		return 1
	}
	return 0
}

func main() {
	ht := hash_grow.NewHashTab[Item](16, 0) // initial size 16, default saturation 0.5

	ht.Insert(&Item{Key: "alpha"})
	ht.Insert(&Item{Key: "beta"})

	if it := ht.Search(&Item{Key: "alpha"}); it != nil {
		fmt.Println("found", it.Key)
	}

	ht.Delete(&Item{Key: "alpha"})

	// Range-over-func iteration (Go 1.23+):
	for pos, item := range ht.All() {
		fmt.Printf("bucket %d: %s\n", pos, item.Key)
	}
	for item := range ht.Values() {
		fmt.Println(item.Key)
	}

	// Callback-style iteration (kept for compatibility):
	ht.Walk(func(pos, depth int, data *Item, userData any) bool {
		fmt.Println(data.Key)
		return true // return false to stop the walk
	}, nil)

	ht.Print(os.Stdout) // one element per line, bucket order
	ht.Truncate()       // remove everything
}
```

## Iteration

Two styles are supported:

- Go 1.23+ range-over-func: `All() iter.Seq2[int, *T]` (bucket position and
  element) and `Values() iter.Seq[*T]`, both in bucket order.
- The legacy `Walk(ApplyFunction[T], userData)` callback API.

## Thread safety

`hash_grow` is **not** safe for concurrent use. Use the `hash_grow_ts`
package, which guards every operation with a `sync.RWMutex`, when the table
is shared between goroutines.
