/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stack_ts

import "iter"

import "slices"

// All returns a range-over-func iterator that yields the index and value
// of each element on the stack, from the top (most recently pushed) to
// the bottom.  The index starts at 0 for the top element.
//
//	for i, v := range stk.All() {
//		...
//	}
//
// The iterator operates on a snapshot copied under the read lock when All
// is called, so it is safe to call other stack operations — including
// from inside the loop — and it never observes later modifications.
// Complexity is O(n).
func (ns *Stack[T]) All() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil stack iterates as an empty one
	}
	ns.lock.RLock()
	snapshot := make([]T, len(ns.data))
	copy(snapshot, ns.data)
	ns.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for i, v := range slices.Backward(snapshot) {
			if !yield(len(snapshot)-1-i, v) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that yields the index and
// value of each element on the stack, from the bottom (least recently
// pushed) to the top.  The index starts at 0 for the bottom element.
//
//	for i, v := range stk.Backward() {
//		...
//	}
//
// The iterator operates on a snapshot copied under the read lock when
// Backward is called, so it is safe to call other stack operations —
// including from inside the loop — and it never observes later
// modifications.
// Complexity is O(n).
func (ns *Stack[T]) Backward() iter.Seq2[int, T] {
	if ns == nil {
		return func(func(int, T) bool) {} // a nil stack iterates as an empty one
	}
	ns.lock.RLock()
	snapshot := make([]T, len(ns.data))
	copy(snapshot, ns.data)
	ns.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for i := range snapshot {
			if !yield(i, snapshot[i]) {
				return
			}
		}
	}
}
