# hash_tab

A generic hash table for Go (1.23+) with separate chaining. Each bucket is a
singly linked list (`github.com/pschlump/pluto/sll`).

Elements are stored as `*T`, where `T` implements
`comparable.Equality` (an `IsEqual(x Equality) bool` method). Keys are hashed
with FNV-32a over the element's `String()` method, or with the element's own
`HashKey` method if it implements the `Hashable` interface.

Duplicate inserts are allowed: the new item is placed at the head of its
bucket, hiding the older one (the bucket acts like a stack). Each `Delete`
removes exactly one copy.

## Complexity

`n` = number of elements, `k` = number of buckets. Bucket chains average
`n/k` in length with a reasonable hash.

| Operation   | Complexity | Notes                                             |
|-------------|------------|---------------------------------------------------|
| `Insert`    | O(1)       | insert at bucket head                             |
| `Search`    | O(n/k)     | hash + walk one bucket chain                      |
| `Delete`    | O(n/k)     | hash + walk one bucket chain                      |
| `IsEmpty`   | O(1)       |                                                   |
| `Len`/`Length` | O(1)    |                                                   |
| `Truncate`  | O(k)       | clears every bucket                               |
| `All`       | O(n)       | range-over-func iterator over all elements        |
| `Dump`      | O(n)       | diagnostic print                                  |

## Example

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/hash_tab"
)

type Item struct {
	Name string
}

func (a Item) IsEqual(x comparable.Equality) bool {
	if b, ok := x.(Item); ok {
		return a.Name == b.Name
	}
	if b, ok := x.(*Item); ok {
		return a.Name == b.Name
	}
	return false
}

func main() {
	ht := hash_tab.NewHashTab[Item](101) // 101 buckets, must be >= 5

	ht.Insert(&Item{Name: "alice"})
	ht.Insert(&Item{Name: "bob"})

	if it := ht.Search(&Item{Name: "alice"}); it != nil {
		fmt.Println("found", it.Name)
	}

	ht.Delete(&Item{Name: "bob"})
	fmt.Println("len:", ht.Len())

	// Range-over-func iteration (Go 1.23+):
	for i, v := range ht.All() {
		fmt.Println(i, v.Name)
	}
}
```

An element type may override the default hashing by implementing:

```go
func (a Item) HashKey(x interface{}) int { /* ... */ }
```

## Thread Safety

`HashTab` is **not** thread safe. If the table is shared between goroutines,
all access must be serialized by the caller (e.g. with a `sync.RWMutex`).
