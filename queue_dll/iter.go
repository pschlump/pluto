/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators over the live list: All follows the next
// pointers from the head (the dequeue order), Backward the prev
// pointers from the tail.  Both cost O(n) time with O(1) extra space
// and no snapshot copy; the queue must not be modified while an
// iterator is running.

package queue_dll

import "iter"

// All returns a range-over-func iterator that yields the index and value
// of each element in the queue, from head (next to be dequeued) to tail.
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
		i := 0
		for p := q.head; p != nil; p = p.next {
			if !yield(i, p.data) {
				return
			}
			i++
		}
	}
}

// Backward returns a range-over-func iterator that yields the index and
// value of each element in the queue, from tail (most recently enqueued)
// to head.
//
//	for i, v := range q.Backward() {
//		...
//	}
//
// The index counts down from Length()-1 to 0, so it matches the index
// that All assigns to the same element.
//
// The queue must not be modified while the iterator is running.
// Complexity is O(n).
func (q *Queue[T]) Backward() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		i := q.length - 1
		for p := q.tail; p != nil; p = p.prev {
			if !yield(i, p.data) {
				return
			}
			i--
		}
	}
}
