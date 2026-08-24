package skip_list_dll

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Go 1.23+ range-over-func iterators for the doubly-linked skip list.
// Because level 0 is doubly-linked, both directions are simple pointer walks
// with O(1) extra space.  Both support early exit via break.

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
// Complexity is O(n) time, O(1) space.
func (tt *SkipList[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt.IsEmpty() {
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
// Complexity is O(n) time, O(1) space.
func (tt *SkipList[T]) Backward() iter.Seq[T] {
	return func(yield func(T) bool) {
		if tt.IsEmpty() {
			return
		}
		for cur := tt.lastNode(); cur != tt.head; cur = cur.prev {
			if !yield(*cur.data) {
				return
			}
		}
	}
}
