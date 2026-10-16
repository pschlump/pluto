/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq

import "iter"

// All returns an iterator over the elements of the heap.  The elements
// are produced in internal heap (breadth-first tree) order — this is NOT
// sorted order; repeatedly calling Pop is the way to consume a heap in
// sorted order.
//
//	for v := range h.All() {
//		fmt.Println(v)
//	}
//
// The heap must not be modified while iterating.
// Complexity is O(n) for a full iteration.
func (dh *DHeap[T]) All() iter.Seq[T] {
	if dh == nil {
		return func(func(T) bool) {} // a nil heap iterates as an empty one
	}
	return func(yield func(T) bool) {
		for _, v := range dh.data {
			if !yield(v) {
				return
			}
		}
	}
}
