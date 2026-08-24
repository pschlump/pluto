// Copyright (C) Philip Schlump, 2012-2023.
// BSD 3 Clause Licensed.

package dqueue_ts

import "iter"

// All returns a range-over-func iterator that yields the index and a pointer
// to each element in the queue, from front to back.
//
//	for i, v := range q.All() {
//		...
//	}
//
// Iteration uses the thread safe step-at-a-time iterator of the underlying
// doubly linked list: each step takes the list lock, so it is safe to use
// concurrently with other operations, but it is not a consistent snapshot —
// elements pushed while iterating may be visited and elements popped by
// another goroutine may be skipped.
// Complexity is O(n).
func (q *Deque[T]) All() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		it := q.data.Front()
		for i := 0; !it.Done(); i++ {
			if !yield(i, it.Value()) {
				return
			}
			it.Next()
		}
	}
}

// Backward returns a range-over-func iterator that yields the index and a
// pointer to each element in the queue, from back to front.
//
//	for i, v := range q.Backward() {
//		...
//	}
//
// The index counts down from Length()-1 to 0, so it matches the index that
// All assigns to the same element.  Iteration is thread safe but is not a
// consistent snapshot; see All for details.
// Complexity is O(n).
func (q *Deque[T]) Backward() iter.Seq2[int, *T] {
	return func(yield func(int, *T) bool) {
		it := q.data.Rear()
		for ; !it.Done(); it.Prev() {
			if !yield(it.Pos(), it.Value()) {
				return
			}
		}
	}
}

/* vim: set noai ts=4 sw=4: */
