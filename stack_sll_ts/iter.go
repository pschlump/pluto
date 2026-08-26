/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators.  Both operate on a snapshot taken when they
// are called; All numbers from 0 at the top (most recently pushed), and
// Backward — because the underlying list is singly linked — reverses the
// snapshot, which costs the same O(n) copy in time and O(n) temporary
// space.

package stack_sll_ts

import "slices"

import "iter"

// All returns an iterator over the elements of the stack from the top
// (most recently pushed) to the bottom.  The index starts at 0 for the
// top element.
//
// The iterator operates on a snapshot taken when All is called, so it is
// safe to call other stack operations — including from inside the loop —
// and it never observes later modifications.
// Complexity is O(n).
func (ns *Stack[T]) All() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil stack iterates as an empty one
	}
	items := ns.snapshot() // top first
	return func(yield func(int, T) bool) {
		for i, v := range items {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements of the stack from the
// bottom (least recently pushed) to the top.  The index starts at 0 for
// the bottom element.
//
// The iterator operates on a snapshot taken when Backward is called, so
// it is safe to call other stack operations — including from inside the
// loop — and it never observes later modifications.
// Complexity is O(n).
func (ns *Stack[T]) Backward() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil stack iterates as an empty one
	}
	items := ns.snapshot() // top first
	return func(yield func(int, T) bool) {
		for i, item := range slices.Backward(items) {
			if !yield(len(items)-1-i, item) {
				return
			}
		}
	}
}
