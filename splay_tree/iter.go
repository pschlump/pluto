/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package splay_tree

import (
	"iter"
)

// All returns an iterator that yields every element of the tree in
// in-order (ascending) order with its ordinal position (0-based).  It is a
// range-over-func iterator:
//
//	for i, v := range tree.All() {
//		...
//	}
//
// Note that a single-variable range yields the INDEX, not the value:
//
//	for i := range tree.All() { // i is the position, not the element
//		...
//	}
//
// The iterator walks the live nodes without splaying; the tree must not be
// modified while the returned iterator is being consumed — and for a splay
// tree every Search, FindMin or FindMax counts as a modification.
// Complexity is O(n) for a full iteration.
func (tt *SplayTree[T]) All() iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		var stk []*SplayTreeElement[T]
		n := tt.root
		i := 0
		for n != nil || len(stk) > 0 {
			for n != nil {
				stk = append(stk, n)
				n = n.left
			}
			n = stk[len(stk)-1]
			stk = stk[:len(stk)-1]
			if !yield(i, n.data) {
				return
			}
			i++
			n = n.right
		}
	}
}

// Backward returns an iterator that yields every element of the tree in
// reverse in-order (descending) order.  Indexes count down from Len()-1,
// so each element carries the same ordinal position it has in All.  As
// with All, a single-variable range yields the INDEX, and the tree must
// not be modified (remember: every access splays) while the iterator is
// being consumed.  It is a range-over-func iterator:
//
//	for i, v := range tree.Backward() {
//		...
//	}
//
// Complexity is O(n) for a full iteration.
func (tt *SplayTree[T]) Backward() iter.Seq2[int, T] {
	if tt == nil {
		return func(func(int, T) bool) {} // a nil tree iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		var stk []*SplayTreeElement[T]
		n := tt.root
		i := tt.length - 1
		for n != nil || len(stk) > 0 {
			for n != nil {
				stk = append(stk, n)
				n = n.right
			}
			n = stk[len(stk)-1]
			stk = stk[:len(stk)-1]
			if !yield(i, n.data) {
				return
			}
			i--
			n = n.left
		}
	}
}
