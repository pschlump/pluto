/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators for the skip list.  All walks the level-0
// chain directly; Backward collects the chain into a slice first, because
// skip list nodes have no back pointers.  Both support early exit via
// break.

package skip_list

import (
	"iter"
	"slices"
)

// All returns an iterator over the items in the list in ascending
// sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.All() {
//		fmt.Println(v)
//	}
//
// The list must not be modified while the iterator is being consumed — it
// walks the live nodes.
// Complexity is O(n) time, O(1) space.
func (tt *SkipList[T]) All() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(T) bool) {
		if tt.IsEmpty() {
			return
		}
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			if !yield(cur.data) {
				return
			}
		}
	}
}

// Backward returns an iterator over the items in the list in descending
// sequence.  It can be used directly in a for/range loop:
//
//	for v := range list.Backward() {
//		fmt.Println(v)
//	}
//
// The items are collected into a slice first, because skip list nodes
// have no back pointers.  The list must not be modified while the
// iterator is being consumed.
// Complexity is O(n) time and O(n) space.
func (tt *SkipList[T]) Backward() iter.Seq[T] {
	if tt == nil {
		return func(func(T) bool) {} // a nil list iterates as an empty one
	}
	return func(yield func(T) bool) {
		if tt.IsEmpty() {
			return
		}
		var items []T
		for cur := tt.head.forward[0]; cur != nil; cur = cur.forward[0] {
			items = append(items, cur.data)
		}
		for _, item := range slices.Backward(items) {
			if !yield(item) {
				return
			}
		}
	}
}
