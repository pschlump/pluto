# rb_tree — Ordered Collection (Red-Black Tree)

Package `rb_tree` implements an ordered collection of generic items as a
[red-black](https://en.wikipedia.org/wiki/Red%E2%80%93black_tree)
self-balancing binary search tree.  Items must implement
`github.com/pschlump/pluto/comparable.Comparable` (a single `Compare`
method).

A red-black tree keeps every root-to-leaf path within a factor of two of the
shortest one, so the ordered operations are `O(log n)` in the **worst
case** — unlike `bst` they do not degrade on sorted input, and unlike
`skip_list` the bound is deterministic rather than expected.  For a
thread-safe version of this package with an identical API, see `rb_tree_ts`.

## Complexity

| Operation                     | Worst case | Notes                                  |
|-------------------------------|------------|----------------------------------------|
| `Insert`                      | O(log n)   | duplicate keys replace stored item     |
| `Search`                      | O(log n)   | returns `nil` when not found           |
| `Delete`                      | O(log n)   | returns `false` when not found         |
| `FindMin`/`FindMax`           | O(log n)   |                                        |
| `DeleteAtHead`/`DeleteAtTail` | O(log n)   | remove smallest/largest                |
| `IsEmpty`/`Length`            | O(1)       |                                        |
| `Truncate`                    | O(1)       | drops the whole tree                   |
| `Depth`                       | O(log n)   | longest root-to-leaf path              |
| `All`/`Backward`              | O(n)       | full traversal, O(1) extra space       |

The iterators use the parent pointers (successor/predecessor walks), so they
need no stack and no snapshot.

## Usage

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/rb_tree"
)

type Item int

func (a Item) Compare(x comparable.Comparable) int {
	b := x.(Item)
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

func main() {
	var tree rb_tree.RbTree[Item]

	tree.Insert(Item(5))
	tree.Insert(Item(2))
	tree.Insert(Item(9))

	if p := tree.Search(Item(2)); p != nil {
		fmt.Println("found", *p)
	}

	// Modern Go 1.23+ range-over-func iteration, ascending order:
	for v := range tree.All() {
		fmt.Println(v) // 2 5 9
	}
	// Descending order:
	for v := range tree.Backward() {
		fmt.Println(v) // 9 5 2
	}

	tree.Delete(Item(5))
	fmt.Println(tree.Length()) // 2
}
```

## Thread Safety

`RbTree` is **not** safe for concurrent use.  Use `rb_tree_ts` instead, or
guard all access with an external mutex.
