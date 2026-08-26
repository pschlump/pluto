/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators for the thread-safe red-black tree.  To remain
// race-free both iterators operate on a snapshot of the tree items taken
// under the read lock when they are called; modifications of the tree
// after that point are not reflected in the iteration, and the lock is
// never held while the loop body runs.  Both support early exit via break.

package rb_tree_ts

import (
	"iter"
	"slices"
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

// All returns an iterator over the items in a snapshot of the tree in
// ascending (in-order) sequence.  It can be used directly in a for/range
// loop:
//
//	for v := range tree.All() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when All is called, so it is safe to call other
// tree operations — including from inside the loop — and the iterator
// never observes later modifications.
// Complexity is O(n).
func (tt *RbTree[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	items := tt.toSlice()
	return func(yield func(T) bool) {
		for _, v := range items {
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in a snapshot of the tree
// in descending (reverse in-order) sequence.  It can be used directly in a
// for/range loop:
//
//	for v := range tree.Backward() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when Backward is called, so it is safe to call
// other tree operations — including from inside the loop — and the
// iterator never observes later modifications.
// Complexity is O(n).
func (tt *RbTree[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil tree iterates as an empty one
	}
	items := tt.toSlice()
	return func(yield func(T) bool) {
		for _, item := range slices.Backward(items) {
			if !yield(item) {
				return
			}
		}
	}
}
