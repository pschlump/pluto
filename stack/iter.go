/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

package stack

import "iter"

// All returns an iterator over the elements of the stack from the top
// (most recently pushed) to the bottom.  The index starts at 0 for the
// top element.
//
// The iterator reflects the live stack: pushing or popping while
// iterating changes the elements seen.
func (ns *Stack[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := len(*ns) - 1; i >= 0; i-- {
			if !yield(len(*ns)-1-i, (*ns)[i]) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements of the stack from the
// bottom (least recently pushed) to the top.  The index starts at 0 for
// the bottom element.
//
// The iterator reflects the live stack: pushing or popping while
// iterating changes the elements seen.
func (ns *Stack[T]) Backward() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i := range *ns {
			if !yield(i, (*ns)[i]) {
				return
			}
		}
	}
}
