// Copyright (C) 2026 Philip Schlump. All rights reserved.

package heap_ts

import "iter"

// All returns an iterator (Go 1.23+ range-over-func) over the elements of the
// heap.  The elements are produced in internal heap (breadth-first tree)
// order — this is NOT sorted order; repeatedly calling Pop is the way to
// consume a heap in sorted order.
//
// The iterator walks a snapshot of the heap taken under the read lock, so it
// is safe to call heap methods (including Push/Pop/Delete) from inside the
// loop body; changes made after the snapshot are not visible to the
// iteration.
//
//	for v := range h.All() {
//		fmt.Println(*v)
//	}
//
// Complexity is O(n) for a full iteration, plus O(n) extra space for the
// snapshot.
func (hp *Heap[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		for _, v := range hp.snapshot() {
			if !yield(v) {
				return
			}
		}
	}
}
