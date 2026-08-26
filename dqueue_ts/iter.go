/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators.  Both operate on a snapshot taken when they
// are called; All walks it front to back, and Backward walks the same
// snapshot in reverse — the doubly linked list makes a backward walk
// natural, but sharing one front-to-back snapshot keeps both iterators
// on identical data for the same cost, O(n) time and O(n) space.

package dqueue_ts

import "slices"

import "iter"

// All returns an iterator over the elements of the deque from the front
// to the back.  The index starts at 0 for the front element.
//
// The iterator operates on a snapshot taken when All is called, so it is
// safe to call other deque operations — including from inside the loop —
// and it never observes later modifications.
// Complexity is O(n).
func (q *Deque[T]) All() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil deque iterates as an empty one
	}
	items := q.snapshot() // front first
	return func(yield func(int, T) bool) {
		for i, v := range items {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the elements of the deque from the
// back to the front.  The index counts down from Length()-1 to 0, so it
// matches the index that All assigns to the same element.
//
// The iterator operates on a snapshot taken when Backward is called, so
// it is safe to call other deque operations — including from inside the
// loop — and it never observes later modifications.
// Complexity is O(n).
func (q *Deque[T]) Backward() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil deque iterates as an empty one
	}
	items := q.snapshot() // front first; walked in reverse below
	return func(yield func(int, T) bool) {
		for i, item := range slices.Backward(items) {
			if !yield(i, item) {
				return
			}
		}
	}
}
