# ex1 — Minimal Generic Binary Search Tree Example

## Overview

`ex1` is a minimal example of a generic binary search tree written with Go
type parameters.  It exists to demonstrate the basics — a generic type, a
constraint (`cmp.Ordered`), and a Go 1.23+ range-over-func iterator — not
to be a production data structure.  For that use `binary_tree` (unbalanced,
full-featured) or `avl_tree` (self-balancing).

## Operations

| Operation | Description                                       | Complexity       |
|-----------|---------------------------------------------------|------------------|
| `Insert`  | Add an item; equal items replace the node's value | O(h), worst O(n) |
| `IsEmpty` | Report whether the tree is empty                  | O(1)             |
| `All`     | Range-over-func iterator, in-order (ascending)    | O(n)             |

`h` is the tree height; the tree is not balanced, so a sorted insertion
sequence degenerates to a linked list.

## Example

```go
var tree ex1.BinaryTree[int]
for _, v := range []int{5, 3, 8, 1, 4} {
	tree.Insert(v)
}
for v := range tree.All() { // 1 3 4 5 8
	fmt.Println(v)
}
```

## Thread Safety

`BinaryTree` is not safe for concurrent use.  Guard it with a mutex or use
the `binary_tree_ts` package.
