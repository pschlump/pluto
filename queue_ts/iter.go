// Copyright (C) Philip Schlump, 2012-2023.
// BSD 3 Clause Licensed.

package queue

import "iter"

// All returns a range-over-func iterator that yields the index and value of
// each element in the queue, from head (next to be dequeued) to tail.
//
//	for i, v := range q.All() {
//		...
//	}
//
// The iterator operates on a consistent snapshot of the queue taken when
// All is called; the lock is not held while the loop body runs, so it is
// safe to call other Queue methods from inside the loop.
// Complexity is O(n).
func (q *Queue[T]) All() iter.Seq2[int, T] {
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
// The iterator operates on a consistent snapshot of the queue taken when
// Backward is called; the lock is not held while the loop body runs, so it is
// safe to call other Queue methods from inside the loop.
// Complexity is O(n).
func (q *Queue[T]) Backward() iter.Seq2[int, T] {
	q.lock.RLock()
	snapshot := make([]T, len(q.data))
	copy(snapshot, q.data)
	q.lock.RUnlock()
	return func(yield func(int, T) bool) {
		for i := len(snapshot) - 1; i >= 0; i-- {
			if !yield(i, snapshot[i]) {
				return
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */
