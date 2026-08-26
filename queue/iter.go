/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package queue

import "slices"

import "iter"

// All returns a range-over-func iterator that yields the index and value of
// each element in the queue, from head (next to be dequeued) to tail.
//
//	for i, v := range q.All() {
//		...
//	}
//
// The queue must not be modified while the iterator is running.
// Complexity is O(n).
func (q *Queue[T]) All() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		for i, v := range q.data {
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
// The queue must not be modified while the iterator is running.
// Complexity is O(n).
func (q *Queue[T]) Backward() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		for i, v := range slices.Backward(q.data) {
			if !yield(i, v) {
				return
			}
		}
	}
}
