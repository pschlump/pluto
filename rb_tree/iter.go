/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators for the tree.  Because every node has a
// parent pointer, both iterators are simple successor/predecessor walks
// with O(1) extra space and no explicit stack.  Both support early exit
// via break.

package rb_tree

import (
	"iter"
)

// successor returns the node holding the next-larger item after n, or nil
// if n holds the largest item.
func successor[T any](n *RbTreeNode[T]) *RbTreeNode[T] {
	if n.right != nil {
		return minNode(n.right)
	}
	for n.parent != nil && n == n.parent.right {
		n = n.parent
	}
	return n.parent
}

// predecessor returns the node holding the next-smaller item before n, or
// nil if n holds the smallest item.
func predecessor[T any](n *RbTreeNode[T]) *RbTreeNode[T] {
	if n.left != nil {
		return maxNode(n.left)
	}
	for n.parent != nil && n == n.parent.left {
		n = n.parent
	}
	return n.parent
}

// All returns an iterator over the items in the tree in ascending
// (in-order) sequence.  It can be used directly in a for/range loop:
//
//	for v := range tree.All() {
//		fmt.Println(v)
//	}
//
// The tree must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n) time, O(1) space.
func (tt *RbTree[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(T) bool) {
		for cur := minNode(tt.root); cur != nil; cur = successor(cur) {
			if !yield(cur.data) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in the tree in descending
// (reverse in-order) sequence.  It can be used directly in a for/range
// loop:
//
//	for v := range tree.Backward() {
//		fmt.Println(v)
//	}
//
// The tree must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n) time, O(1) space.
func (tt *RbTree[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(T) bool) {
		for cur := maxNode(tt.root); cur != nil; cur = predecessor(cur) {
			if !yield(cur.data) {
				return
			}
		}
	}
}
