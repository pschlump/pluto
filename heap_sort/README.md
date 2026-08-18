# heap_sort — sorting via a generic min-heap

A sorting facility built on top of the generic min-heap in
`github.com/pschlump/pluto/heap`.  Elements (any type implementing
`github.com/pschlump/pluto/comparable.Comparable`) are added with `Insert`
or `InsertArray` and extracted in ascending order with `Sort` or descending
order with `SortDown`.  Sorting drains the underlying heap.

Use `NewHeapSort[T]()` to create a sorter.

## Operations

| Operation | Description | Complexity |
|---|---|---|
| `Insert(x)` | Add a single element | O(log n) |
| `InsertArray(x)` | Add a slice of elements, then rebuild the heap | O(m + n) |
| `Sort()` | Remove all elements, returned in ascending order | O(n log n) |
| `SortDown()` | Remove all elements, returned in descending order | O(n log n) |
| `Len()` / `Length()` | Number of pending elements | O(1) |
| `Truncate()` | Remove all pending elements | O(1) |

`Sort` and `SortDown` empty the sorter; it can be reused afterwards.

## Example

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap_sort"
)

type item int

func (a item) Compare(x comparable.Comparable) int {
	return int(a) - int(x.(item))
}

func main() {
	hs := heap_sort.NewHeapSort[item]()
	for _, v := range []item{5, 2, 8, 1} {
		v := v
		hs.Insert(&v)
	}

	sorted := hs.Sort() // ascending
	for _, v := range sorted {
		fmt.Print(*v, " ") // 1 2 5 8
	}
}
```

## Thread safety

This implementation is *not* thread safe.  There is currently no
mutex-guarded `_ts` variant of this package; guard it externally with a
`sync.Mutex` if it is shared between goroutines.
