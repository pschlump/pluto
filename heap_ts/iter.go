/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package heap_ts

import "iter"

// All returns an iterator over a snapshot of the heap's elements.  The
// elements are produced in internal heap (breadth-first tree) order —
// this is NOT sorted order; repeatedly calling Pop is the way to consume
// a heap in sorted order.
//
//	for v := range h.All() {
//		fmt.Println(v)
//	}
//
// The snapshot is taken when All is called, so it is safe to call other
// heap operations — including from inside the loop — and it never
// observes later modifications.
// Complexity is O(n) for a full iteration.
func (hp *Heap[T]) All() iter.Seq[T] {
	if hp == nil {
		return func(func(T) bool) {} // a nil heap iterates as an empty one
	}
	items := hp.snapshot()
	return func(yield func(T) bool) {
		for _, v := range items {
			if !yield(v) {
				return
			}
		}
	}
}
