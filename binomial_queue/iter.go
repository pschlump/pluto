/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Range-over-func iterators.  Both walk the live forest — All in
// pre-order over the trees in forest order, Backward in the exact
// reverse — so both cost O(n) time with O(1) extra space and no snapshot
// copy.  The price is the usual one for a plain package: the queue must
// not be modified while an iterator is running.

package binomial_queue

import "iter"

// All returns a range-over-func iterator that yields the index and value
// of each element of the queue.  The elements are produced in internal
// forest order (pre-order within each tree) — this is NOT sorted order;
// repeatedly calling DeleteMin is the way to consume a queue in sorted
// order.  The index counts up from 0 in iteration order (a
// single-variable range yields the index).
//
//	for i, v := range q.All() {
//		...
//	}
//
// The queue must not be modified while the iterator is running.
// Complexity is O(n) for a full iteration.
func (q *BinomialQueue[T]) All() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		i := 0
		var walk func(n *bqNode[T]) bool
		walk = func(n *bqNode[T]) bool {
			if !yield(i, n.value) {
				return false
			}
			i++
			for _, c := range n.children {
				if !walk(c) {
					return false
				}
			}
			return true
		}
		for _, tr := range q.trees {
			if !walk(tr) {
				return
			}
		}
	}
}

// Backward returns a range-over-func iterator that yields the index and
// value of each element of the queue in the exact reverse of the order
// that All produces.  The index counts down from Length()-1 to 0, so it
// matches the index that All assigns to the same element.
//
//	for i, v := range q.Backward() {
//		...
//	}
//
// The queue must not be modified while the iterator is running.
// Complexity is O(n) for a full iteration.
func (q *BinomialQueue[T]) Backward() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	return func(yield func(int, T) bool) {
		i := q.length - 1
		var walk func(n *bqNode[T]) bool
		walk = func(n *bqNode[T]) bool {
			for k := len(n.children) - 1; k >= 0; k-- {
				if !walk(n.children[k]) {
					return false
				}
			}
			if !yield(i, n.value) {
				return false
			}
			i--
			return true
		}
		for k := len(q.trees) - 1; k >= 0; k-- {
			if !walk(q.trees[k]) {
				return
			}
		}
	}
}
