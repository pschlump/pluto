# binary_tree_ts

Package `binary_tree_ts` implements a generic, **unbalanced** binary search
tree that is safe for concurrent use. Every operation is guarded by a
`sync.RWMutex`. It exposes exactly the same API as
[`binary_tree`](../binary_tree) — see that package for a non-thread-safe
version without the locking overhead. For a self-balancing tree with
guaranteed `O(log n)` operations see [`avl_tree`](../avl_tree).

## Operations and Complexity

Because the tree is not self-balancing, the "average" column assumes randomly
ordered input; sorted input degenerates to a linked list.

| Operation                                                    | Average    | Worst case | Notes                                              |
|--------------------------------------------------------------|------------|------------|----------------------------------------------------|
| `Insert`                                                     | O(log₂ n)  | O(n)       | duplicate replaces existing element, returns false |
| `Search`                                                     | O(log₂ n)  | O(n)       | returns nil when not found                         |
| `Delete` / `DeleteMatch`                                     | O(log₂ n)  | O(n)       | returns false when not found                       |
| `FindMin` / `FindMax`                                        | O(log₂ n)  | O(n)       | nil on an empty tree                               |
| `DeleteAtHead` / `DeleteAtTail`                              | O(log₂ n)  | O(n)       | removes the min / max element                      |
| `IsEmpty`, `Len`, `Length`                                   | O(1)       | O(1)       |                                                    |
| `Truncate`                                                   | O(1)       | O(1)       | drops the whole tree                               |
| `Index`                                                      | O(n)       | O(n)       | Nth element in in-order order                      |
| `Depth`                                                      | O(n)       | O(n)       | levels in the deepest part; empty tree is 0        |
| `Reverse`                                                    | O(n)       | O(n)       | swaps left/right children of every node            |
| `WalkInOrder` / `WalkPreOrder` / `WalkPostOrder`, `WalkFunc` | O(n)       | O(n)       | callback traversals                                |
| `Front` iterator, `All`, `Backward`                          | O(n) total | O(n) total | see Iteration below                                |

## Usage

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/binary_tree_ts"
	"github.com/pschlump/pluto/comparable"
)

type Str string

func (s Str) Compare(x comparable.Comparable) int {
	o := x.(Str)
	if s < o {
		return -1
	} else if s > o {
		return 1
	}
	return 0
}

func main() {
	tree := binary_tree_ts.NewBinaryTree[Str]()
	bob, carol := Str("bob"), Str("carol")
	tree.Insert(&bob)
	tree.Insert(&carol)

	found := tree.Search(&carol) // *Str, or nil
	fmt.Println(*found)

	// Range-over-func iteration (Go 1.23+), in ascending order:
	for v := range tree.All() {
		fmt.Println(*v)
	}

	tree.Delete(&bob)
}
```

## Iteration

Three styles are supported, all kept for compatibility:

- `All()` / `Backward()` — Go 1.23 range-over-func iterators
  (`iter.Seq[*T]`) in ascending / descending order. Preferred for new code.
- `Front()` with `Value()` / `Next()` / `Done()` — old-style in-order
  iterator.
- `WalkInOrder` / `WalkPreOrder` / `WalkPostOrder` / `WalkFunc` —
  callback-based traversals; returning `false` from an `ApplyFunction`
  callback stops the walk.

## Thread Safety

`BinaryTree` **is** safe for concurrent use: every operation takes the
tree's `sync.RWMutex` (read operations take the read lock).

Caveats:

- The iterators (`Front`, `All`, `Backward`) operate on a **snapshot** of the
  tree taken when the iterator is created, so it is safe to call other tree
  operations (including `Insert`/`Delete`) from inside a loop; the iteration
  does not observe those modifications.
- The `Walk*` callbacks run while the tree's read lock is held. A callback
  must not call back into the same tree (that would deadlock) and should be
  fast so it does not block writers.
- The returned `*T` values alias the data stored in the tree; mutating a
  returned value concurrently is the caller's responsibility.
