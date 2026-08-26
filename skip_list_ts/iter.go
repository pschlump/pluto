/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators for the thread-safe skip list.  To remain
// race-free both iterators operate on a snapshot of the list data taken
// under the read lock when they are called; modifications of the list
// after that point are not reflected in the iteration, and the lock is
// never held while the loop body runs.  Both support early exit via
// break.

package skip_list_ts

import (
	"iter"
	"slices"
)

// All returns an iterator over the items in a snapshot of the list in
// ascending sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.All() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when All is called, so it is safe to call other
// list operations — including from inside the loop — and the iterator
// never observes later modifications.
// Complexity is O(n).
func (tt *SkipList[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
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

// Backward returns an iterator over the items in a snapshot of the list
// in descending sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.Backward() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when Backward is called, so it is safe to call
// other list operations — including from inside the loop — and the
// iterator never observes later modifications.
// Complexity is O(n).
func (tt *SkipList[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
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
