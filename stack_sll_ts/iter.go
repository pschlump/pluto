/*
Copyright (C) Philip Schlump, 2023.

BSD 3 Clause Licensed.
*/

package stack

import "iter"

// All returns an iterator over the elements of the stack from the top
// (most recently pushed) to the bottom.  The index starts at 0 for the
// top element.
//
// The iterator walks the live list without holding the stack lock, so it
// is not safe to modify the stack from another goroutine while iterating.
func (ns *Stack[T]) All() iter.Seq2[int, *T] {
	return ns.data.IteratePtr()
}

// Backward returns an iterator over the elements of the stack from the
// bottom (least recently pushed) to the top.  The index starts at 0 for
// the bottom element.  Because the underlying list is singly linked this
// takes O(n) time and O(n) temporary space to set up.
//
// The iterator operates on a snapshot of the element pointers taken when
// iteration starts.
func (ns *Stack[T]) Backward() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		var elems []*T
		for _, p := range ns.data.IteratePtr() {
			elems = append(elems, p)
		}
		for i := len(elems) - 1; i >= 0; i-- {
			if !yield(len(elems)-1-i, elems[i]) {
				return
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */
