# skip_list — Ordered Collection (Skip List)

Package `skip_list` implements an ordered collection of generic items as a
[skip list](https://en.wikipedia.org/wiki/Skip_list): a probabilistic data
structure with multiple levels of linked lists, where each higher level acts
as an "express lane" that skips over roughly half of the nodes below it.
Items must implement `github.com/pschlump/pluto/comparable.Comparable` (a
single `Compare` method).

Unlike an unbalanced binary search tree (see `bst`), a skip list does not
degrade when items are inserted in sorted order.  For a deterministic
balanced alternative, see `avl_tree`.  For a thread-safe version of this
package with an identical API, see `skip_list_ts`.

## Complexity

Skip lists are probabilistic: the expected cost of the ordered operations is
`O(log n)`, with an `O(n)` worst case.

| Operation                     | Expected | Worst case | Notes                              |
|-------------------------------|----------|------------|------------------------------------|
| `Insert`                      | O(log n) | O(n)       | duplicate keys replace stored item |
| `Search`                      | O(log n) | O(n)       | returns `nil` when not found       |
| `Delete`                      | O(log n) | O(n)       | returns `false` when not found     |
| `FindMin`                     | O(1)     | O(1)       | first node on level 0              |
| `FindMax`                     | O(log n) | O(n)       |                                    |
| `DeleteAtHead`                | O(1)     | O(log n)   | remove smallest                    |
| `DeleteAtTail`                | O(log n) | O(n)       | remove largest                     |
| `IsEmpty`/`Length`            | O(1)     | O(1)       |                                    |
| `Truncate`                    | O(1)     | O(1)       | drops the whole list               |
| `All`/`Backward`              | O(n)     | O(n)       | full traversal                     |

## Usage

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/skip_list"
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
	var list skip_list.SkipList[Item]

	list.Insert(Item(5))
	list.Insert(Item(2))
	list.Insert(Item(9))

	if p := list.Search(Item(2)); p != nil {
		fmt.Println("found", *p)
	}

	// Modern Go 1.23+ range-over-func iteration, ascending order:
	for v := range list.All() {
		fmt.Println(v) // 2 5 9
	}
	// Descending order:
	for v := range list.Backward() {
		fmt.Println(v) // 9 5 2
	}

	list.Delete(Item(5))
	fmt.Println(list.Length()) // 2
}
```

## Thread Safety

`SkipList` is **not** safe for concurrent use.  Use `skip_list_ts` instead,
or guard all access with an external mutex.
