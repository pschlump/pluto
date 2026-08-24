package skip_list_dll_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Go 1.23+ range-over-func iterators for the thread-safe doubly-linked skip
// list.  Because level 0 is doubly-linked, both directions are simple
// pointer walks with O(1) extra space — no snapshot is needed.  Both
// iterators hold the read lock for the whole iteration, so the loop body
// must not call back into the same list (the same rule that applies to the
// Walk* callbacks in avl_tree_ts).  Both support early exit via break.

import (
	"iter"
)

// All returns an iterator over the items in the list in ascending sequence.
// It can be used directly in a for/range loop:
//
//	for v := range list.All() {
//		fmt.Println(v)
//	}
//
// The read lock is held for the whole iteration; the loop body must not call
// back into the same list.  Complexity is O(n) time, O(1) space.
func (tt *SkipList[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		tt.lock.RLock()
		defer tt.lock.RUnlock()
		if tt.isEmpty() {
			return
		}
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			if !yield(*cur.data) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in the list in descending
// sequence, walking the level-0 back pointers from the tail.  It can be used
// directly in a for/range loop:
//
//	for v := range list.Backward() {
//		fmt.Println(v)
//	}
//
// The read lock is held for the whole iteration; the loop body must not call
// back into the same list.  Complexity is O(n) time, O(1) space.
func (tt *SkipList[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		tt.lock.RLock()
		defer tt.lock.RUnlock()
		if tt.isEmpty() {
			return
		}
		for cur := tt.lastNode(); cur != tt.head; cur = cur.prev {
			if !yield(*cur.data) {
				return
			}
		}
	}
}
