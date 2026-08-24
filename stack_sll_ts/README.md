# stack_sll_ts — a thread-safe generics based Go Stack

A generic, thread-safe LIFO stack with `Push`, `Pop`, `Peek`, `Length`,
`IsEmpty` and `Truncate`, built on top of the thread-safe singly linked
list [`sll_ts`](../sll_ts).  Elements are stored and returned by pointer.
Go 1.23+ range-over-func iterators (`All`, `Backward`) are provided for
walking the stack.

The zero value is an empty stack, ready to use.  Elements must implement
the [`comparable.Equality`](../comparable) interface (a requirement
inherited from `sll_ts`).

## Complexity

| Operation  | Cost | Notes                                        |
|------------|------|----------------------------------------------|
| `Push`     | O(1) | inserts at the head of the list              |
| `Pop`      | O(1) | returns `sll_ts.ErrEmptySll` when empty      |
| `Peek`     | O(1) | returns the top element pointer              |
| `IsEmpty`  | O(1) |                                              |
| `Length`   | O(1) |                                              |
| `Truncate` | O(1) | unlinks all nodes                            |
| `All`      | O(n) | iterates top to bottom                       |
| `Backward` | O(n) | iterates bottom to top; O(n) temporary space |

## Example

```go
package main

import (
	"fmt"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/stack_sll_ts"
)

type Item struct{ Name string }

func (a Item) IsEqual(x comparable.Equality) bool {
	b, ok := x.(Item)
	return ok && a.Name == b.Name
}

func main() {
	var stk stack.Stack[Item]

	stk.Push(&Item{Name: "a"})
	stk.Push(&Item{Name: "b"})

	for i, v := range stk.All() { // b, a (top to bottom)
		fmt.Println(i, v.Name)
	}

	for !stk.IsEmpty() {
		v, _ := stk.Pop()
		fmt.Println(v.Name) // b, then a
	}
}
```

## Thread Safety

All operations are guarded by a mutex inherited from `sll_ts.Sll` and are
safe for concurrent use.  The `All`/`Backward` iterators walk the live list
without holding the lock for the whole iteration, so do not mutate the
stack concurrently while iterating.
