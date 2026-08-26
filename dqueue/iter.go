/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators.  Unlike the dqueue_ts twins these walk the
// live list — All follows the next pointers from the head, Backward the
// prev pointers from the tail (that backward walk is the reason the
// deque is doubly linked) — so both cost O(n) time with O(1) extra
// space and no snapshot copy.  The price is the usual one for a plain
// package: the deque must not be modified while an iterator is running.

package dqueue

import "iter"

// All returns a range-over-func iterator that yields the index and value
// of each element of the deque, from the front (index 0) to the back.
//
//	for i, v := range q.All() {
//		...
//	}
//
// The deque must not be modified while the iterator is running.
// Complexity is O(n).
func (q *Deque[T]) All() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil deque iterates as an empty one
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
// value of each element of the deque, from the back to the front.  The
// index counts down from Length()-1 to 0, so it matches the index that
// All assigns to the same element.
//
//	for i, v := range q.Backward() {
//		...
//	}
//
// The deque must not be modified while the iterator is running.
// Complexity is O(n).
func (q *Deque[T]) Backward() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil deque iterates as an empty one
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
