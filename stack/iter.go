/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stack

import "slices"

import "iter"

// All returns an iterator over the elements of the stack from the top
// (most recently pushed) to the bottom.  The index starts at 0 for the
// top element.
//
//	The iterator reflects the live stack: pushing or popping while
//	iterating changes the elements seen.
//
// Complexity is O(n).
func (ns *Stack[T]) All() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil stack iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		for i, v := range slices.Backward(ns.data) {
			if !yield(len(ns.data)-1-i, v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements of the stack from the
// bottom (least recently pushed) to the top.  The index starts at 0 for
// the bottom element.
//
//	The iterator reflects the live stack: pushing or popping while
//	iterating changes the elements seen.
//
// Complexity is O(n).
func (ns *Stack[T]) Backward() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil stack iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		for i := range ns.data {
			if !yield(i, ns.data[i]) {
				return
			}
		}
	}
}
