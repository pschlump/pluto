// Package priority_queue implements a generic priority queue backed by a
// binary min-heap.
//
// The queue supports the following operations:
//
//  1. Peek           - look at the lowest-priority (minimum) element
//  2. Insert         - add an element
//  3. Delete         - remove the element at a given position
//  4. Pop            - Peek + Delete of the minimum element
//  5. UpdatePriority - replace the element at a given position and re-heapify
//  6. Search         - find an element by value
//
// Elements are stored as *T and must implement comparable.Comparable.
// The queue is NOT safe for concurrent use; guard it with a mutex if it is
// shared between goroutines.
package priority_queue

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap"
)

// PriorityQueue is a priority queue of *T values ordered by
// comparable.Comparable.Compare. The zero value is not usable;
// create one with NewPriorityQueue.
type PriorityQueue[T comparable.Comparable] struct {
	h *heap.Heap[T]
}

// NewPriorityQueue creates a new, empty priority queue and returns it.
// Complexity is O(1).
func NewPriorityQueue[T comparable.Comparable]() (rv *PriorityQueue[T]) {
	// We don't have to "heapify" at this point because we start all heaps with an empty set of data.
	rv = &PriorityQueue[T]{}
	rv.h = heap.NewHeap[T]()
	return
}

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Len() int {
	return pq.h.Len()
}

// IsEmpty returns true if the queue has no elements.
// Complexity is O(1).
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.h.Len() == 0
}

// Peek returns the minimum element in the queue without removing it,
// or nil if the queue is empty.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Peek() (rv *T) {
	return pq.h.Peek()
}

// Insert adds the element n to the queue.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Insert(n *T) {
	pq.h.Push(n)
}

// Pop removes and returns the minimum element in the queue,
// or nil if the queue is empty.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Pop() (rv *T) {
	return pq.h.Pop()
}

// Search finds the first element that compares equal (Compare == 0) to
// cmpVal and returns it along with its position in the underlying heap.
// If no element matches, rv is nil and err is non-nil.
// Complexity is O(n).
func (pq *PriorityQueue[T]) Search(cmpVal *T) (rv *T, pos int, err error) {
	rv, pos, _ = pq.h.Search(cmpVal)
	if rv == nil {
		err = fmt.Errorf("not found")
	}
	return
}

// UpdatePriority replaces the element at position pos (a position previously
// returned by Search) with newVal and re-establishes the heap ordering.
// It returns true if pos was valid, false otherwise.
//
// Note: heap.Heap.Fix is broken (it can panic with an index-out-of-range in
// down()), so this rebuilds the queue instead of sifting in place.
// Complexity is O(n log n).
func (pq *PriorityQueue[T]) UpdatePriority(pos int, newVal *T) (found bool) {
	n := pq.h.Len()
	if pos < 0 || pos >= n {
		return false
	}
	nh := heap.NewHeap[T]()
	for i := 0; i < n; i++ {
		if i == pos {
			nh.Push(newVal)
		} else {
			nh.Push(pq.h.GetValue(i))
		}
	}
	pq.h = nh
	return true
}

// Delete removes the element at position pos (a position previously returned
// by Search) from the queue. It returns a non-nil error if pos is out of
// range.
//
// Note: heap.Heap.Delete is broken (it removes the wrong element), so this
// rebuilds the queue from the remaining elements instead.
// Complexity is O(n log n).
func (pq *PriorityQueue[T]) Delete(pos int) (err error) {
	n := pq.h.Len()
	if pos < 0 || pos >= n {
		return fmt.Errorf("failed to delete, position %d out of range [0..%d)", pos, n)
	}
	nh := heap.NewHeap[T]()
	for i := 0; i < n; i++ {
		if i != pos {
			nh.Push(pq.h.GetValue(i))
		}
	}
	pq.h = nh
	return nil
}

// Truncate removes all data from the queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Truncate() {
	pq.h.Truncate()
}

// All returns an iterator (Go 1.23+ range-over-func) that yields the
// elements of the queue in priority order, minimum element first.
//
// The iteration is non-destructive: it operates on a private copy of the
// heap, so the queue is unchanged afterwards. It is a snapshot — elements
// inserted or removed during iteration do not affect the sequence.
// Complexity is O(n log n) to build the snapshot, then O(1) per element
// amortized.
//
//	for item := range pq.All() {
//		fmt.Println(item)
//	}
func (pq *PriorityQueue[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		n := pq.h.Len()
		snapshot := heap.NewHeap[T]()
		for i := 0; i < n; i++ {
			snapshot.Push(pq.h.GetValue(i))
		}
		for {
			v := snapshot.Pop()
			if v == nil {
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}
