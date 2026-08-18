# AVL Tree

`avl_tree` is a generic, self-balancing AVL binary search tree for Go 1.23+.
The heights of the two child subtrees of every node are kept within one of
each other, so searches stay fast regardless of insertion order.  See
[https://en.wikipedia.org/wiki/AVL_tree](https://en.wikipedia.org/wiki/AVL_tree)
for the theory.

The element type `T` must implement `comparable.Comparable` (a
`Compare(T) int` method); elements are stored as `*T`.

This is the **not** thread-safe version.  For concurrent use see
[`../avl_tree_ts`](../avl_tree_ts), which exposes the identical API guarded
by a `sync.RWMutex`.

## Operations

| Operation | Description | Complexity |
|---|---|---|
| `Insert` | Add an element; duplicates replace the old data | O(log n) |
| `Delete` | Remove an element; reports whether it was found | O(log n) |
| `Search` | Return the stored element equal to the probe, or nil | O(log n) |
| `FindMin` / `FindMax` | Return the smallest / largest element | O(log n) |
| `DeleteAtHead` / `DeleteAtTail` | Remove the smallest / largest element | O(log n) |
| `Index` | Return the Nth element in in-order sequence | O(n) |
| `IsEmpty` / `Length` / `Depth` | Tree size and height queries | O(1) |
| `Truncate` | Remove all elements | O(1) |
| `Reverse` | Mirror the tree (mainly useful for testing) | O(n) |
| `WalkInOrder` / `WalkPreOrder` / `WalkPostOrder` | Apply a function to every node | O(n) |
| `Copy` / `Union` / `Minus` / `Intersect` | Whole-tree set operations | O(n log n) |
| `Front` + `Next`/`Value`/`Done` | Old-style in-order iterator | O(n) total |
| `All` / `Backward` | Go 1.23 range-over-func iterators | O(n) |

## Usage

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/avl_tree"
	"github.com/pschlump/pluto/comparable"
)

type Key struct {
	S string
}

// Compare implements comparable.Comparable.
func (a Key) Compare(x comparable.Comparable) int {
	b := x.(Key)
	if a.S < b.S {
		return -1
	} else if a.S > b.S {
		return 1
	}
	return 0
}

func main() {
	var tree avl_tree.AvlTree[Key]

	tree.Insert(&Key{S: "05"})
	tree.Insert(&Key{S: "02"})
	tree.Insert(&Key{S: "09"})

	if found := tree.Search(&Key{S: "02"}); found != nil {
		fmt.Println("found:", found.S)
	}

	// In-order (sorted) iteration, Go 1.23 range-over-func style.
	for item := range tree.All() {
		fmt.Println(item.S)
	}

	tree.Delete(&Key{S: "05"})
}
```

## Iteration

Three ways to walk the tree, all in in-order (sorted) sequence:

- `All()` / `Backward()` — modern `iter.Seq[*T]` range-over-func iterators;
  prefer these for new code.  `Backward()` iterates in descending order.
- `Front()` — old-style iterator with `Value()`/`Next()`/`Done()`.
- `WalkInOrder`/`WalkPreOrder`/`WalkPostOrder` — callback-based walks whose
  callback can stop the walk early by returning false.

It is not safe to modify the tree while an iterator or walk is in progress.

## Thread Safety

This package performs **no locking** and is not safe for concurrent use.
Use `github.com/pschlump/pluto/avl_tree_ts` when the tree is shared between
goroutines; it has the identical API.

## License

Copyright (C) Philip Schlump, 2012-2021.  BSD 3 Clause Licensed.
