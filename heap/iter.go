// Copyright (C) 2026 Philip Schlump. All rights reserved.

package heap

import "iter"

// All returns an iterator (Go 1.23+ range-over-func) over the elements of the
// heap.  The elements are produced in internal heap (breadth-first tree)
// order — this is NOT sorted order; repeatedly calling Pop is the way to
// consume a heap in sorted order.
//
// The heap must not be modified while iterating.
//
//	for v := range h.All() {
//		fmt.Println(*v)
//	}
//
// Complexity is O(n) for a full iteration.
func (hp *Heap[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		for _, v := range hp.data {
			if !yield(v) {
				return
			}
		}
	}
}
