/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package queue_ts

import "slices"

import "iter"

// All returns a range-over-func iterator that yields the index and value of
// each element in the queue, from head (next to be dequeued) to tail.
//
//	for i, v := range q.All() {
//		...
//	}
//
// The iterator operates on a snapshot copied under the read lock when All
// is called, so it is safe to call other queue operations — including
// from inside the loop — and it never observes later modifications.
// Complexity is O(n).
func (q *Queue[T]) All() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	q.lock.RLock()
	snapshot := make([]T, len(q.data))
	copy(snapshot, q.data)
	q.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for i, v := range snapshot {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that yields the index and value
// of each element in the queue, from tail (most recently enqueued) to head.
//
//	for i, v := range q.Backward() {
//		...
//	}
//
// The iterator operates on a snapshot copied under the read lock when
// Backward is called, so it is safe to call other queue operations —
// including from inside the loop — and it never observes later
// modifications.
// Complexity is O(n).
func (q *Queue[T]) Backward() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	q.lock.RLock()
	snapshot := make([]T, len(q.data))
	copy(snapshot, q.data)
	q.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for i, s := range slices.Backward(snapshot) {
			if !yield(i, s) {
				return
			}
		}
	}
}
