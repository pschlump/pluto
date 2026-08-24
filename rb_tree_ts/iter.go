package rb_tree_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Go 1.23+ range-over-func iterators for the thread-safe red-black tree.  To
// remain race-free both iterators operate on a snapshot of the tree data
// taken under the read lock when iteration starts; concurrent modifications
// of the tree after that point are not reflected in the iteration, and the
// lock is never held while the loop body runs.  Both support early exit via
// break.

import (
	"iter"
)

// All returns an iterator over the items in a snapshot of the tree in
// ascending (in-order) sequence.  It can be used directly in a for/range
// loop:
//
//	for v := range tree.All() {
//		fmt.Println(v)
//	}
//
// Complexity is O(n).
func (tt *RbTree[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range tt.toSlice() {
			if !yield(v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in a snapshot of the tree in
// descending (reverse in-order) sequence.  It can be used directly in a
// for/range loop:
//
//	for v := range tree.Backward() {
//		fmt.Println(v)
//	}
//
// Complexity is O(n).
func (tt *RbTree[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		items := tt.toSlice()
		for i := len(items) - 1; i >= 0; i-- {
			if !yield(items[i]) {
				return
			}
		}
	}
}
