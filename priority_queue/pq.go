/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package priority_queue implements a generic priority queue backed by a
// binary min-heap (pluto/heap).  The lowest-priority (minimum) element
// is always the next to come out.
//
// Elements are stored and returned by value, and the not-found and
// out-of-range outcomes are reported with false returns instead of nil
// pointers and errors.
//
// The queue supports the following operations:
//
//	Peek           — look at the lowest-priority (minimum) element.		O(1)
//	Insert         — add an element.										O(log n)
//	Pop            — remove and return the minimum element.				O(log n)
//	Delete         — remove the element at a given position.				O(log n)
//	UpdatePriority — replace the element at a position and re-heapify.	O(log n)
//	Search         — find an element by value.							O(n)
//	All            — iterate in priority order (non-destructive).		O(n log n)
//	Len / Length / IsEmpty / Truncate — size and reset.					O(1)
//
// The element type never implements an interface: queues of naturally
// ordered types (all integers, floats and strings) are created with
// NewPriorityQueue, which orders by the built-in < and > operators;
// queues of any other type — including structs ordered by a priority
// field — are created with NewPriorityQueueFunc, which takes a caller
// supplied comparison function.
//
// A nil *PriorityQueue and the zero value both behave as an empty queue
// for every operation except Insert, which panics with a message naming
// the constructors.  The package panics in exactly three situations, all
// programmer errors:
//
//	NewPriorityQueueFunc(nil)      — nil comparison function, caught at construction.
//	Insert on a nil queue          — a nil queue cannot store an element.
//	Insert on a zero-value queue   — no underlying heap; the message names the constructors.
//
// The queue is NOT safe for concurrent use; a mutex-guarded twin has the
// same interface.
package priority_queue

import (
	"cmp"
	"iter"

	"github.com/pschlump/pluto/heap"
)

// PriorityQueue is a priority queue of T values ordered by its comparison
// function.  Use NewPriorityQueue for naturally ordered element types
// (numbers, strings) or NewPriorityQueueFunc for a caller supplied
// comparison function.  The zero value is an empty queue for reads, but
// Insert on it panics — create queues with the constructors.
type PriorityQueue[T any] struct {
	h *heap.Heap[T]

	// cmp is the same comparison function the heap was built with, kept
	// so All can build an ordered snapshot heap with the queue's
	// ordering.
	cmp func(a, b T) int
}

// NewPriorityQueue creates a new, empty priority queue for any naturally
// ordered element type (all integers, floats and strings — cmp.Ordered).
// Ordering uses the built-in < and > operators of T.
// Complexity is O(1).
func NewPriorityQueue[T cmp.Ordered]() *PriorityQueue[T] {
	return newPQ(heap.Compare[T])
}

// NewPriorityQueueFunc creates a new, empty priority queue that orders
// elements with the caller supplied comparison function fx — for example
// a struct ordered by its priority field.
// It panics if fx is nil.
// Complexity is O(1).
func NewPriorityQueueFunc[T any](fx func(a, b T) int) *PriorityQueue[T] {
	if fx == nil {
		panic("priority_queue: NewPriorityQueueFunc called with a nil comparison function")
	}
	return newPQ[T](fx)
}

func newPQ[T any](cmp func(a, b T) int) *PriorityQueue[T] {
	return &PriorityQueue[T]{h: heap.NewHeapFunc(cmp), cmp: cmp}
}

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Len() int {
	if pq == nil || pq.h == nil {
		return 0
	}
	return pq.h.Len()
}

// Length is an alias for Len; it returns the number of elements in the
// queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Length() int {
	return pq.Len()
}

// IsEmpty returns true if the queue has no elements.
// Complexity is O(1).
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.Len() == 0
}

// Peek returns the minimum element in the queue without removing it, or
// false if the queue is empty.
//
// The element is returned by value; it does not alias the queue's
// internals.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Peek() (rv T, found bool) {
	if pq == nil || pq.h == nil {
		return
	}
	return pq.h.Peek()
}

// Insert adds the element n to the queue.
// It panics on a nil queue or on a zero-value queue (no underlying
// heap); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty queue.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Insert(n T) {
	if pq == nil || pq.h == nil {
		panic("priority_queue: Insert called on a nil or zero-value queue (create the queue with NewPriorityQueue or NewPriorityQueueFunc)")
	}
	pq.h.Push(n)
}

// Pop removes and returns the minimum element in the queue, or false if
// the queue is empty.
//
// The element is returned by value; the queue no longer holds any
// reference to it.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Pop() (rv T, found bool) {
	if pq == nil || pq.h == nil {
		return
	}
	return pq.h.Pop()
}

// Search finds the first element that compares equal to cmpVal and
// returns it along with its position in the underlying heap (usable with
// UpdatePriority and Delete), or (zero, -1, false) if no element matches.
// The probe only needs the fields that the comparison function reads.
// Complexity is O(n).
func (pq *PriorityQueue[T]) Search(cmpVal T) (rv T, pos int, found bool) {
	if pq == nil || pq.h == nil {
		return
	}
	return pq.h.Search(cmpVal)
}

// UpdatePriority replaces the element at position pos (a position
// previously returned by Search) with newVal and re-establishes the heap
// ordering.  It reports false and does nothing if pos is out of range.
// Replacing in place is cheaper than a Delete followed by an Insert.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) UpdatePriority(pos int, newVal T) bool {
	if pq == nil || pq.h == nil {
		return false
	}
	return pq.h.Fix(pos, newVal)
}

// Delete removes and returns the element at position pos (a position
// previously returned by Search) from the queue.  It reports false if pos
// is out of range.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Delete(pos int) (rv T, found bool) {
	if pq == nil || pq.h == nil {
		return
	}
	return pq.h.Delete(pos)
}

// Truncate removes all data from the queue.  The comparison function is
// kept, so the queue remains usable and can simply be refilled.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Truncate() {
	if pq == nil || pq.h == nil {
		return
	}
	pq.h.Truncate()
}

// Lock is a no-op provided for API compatibility with a thread-safe
// priority queue twin.  This implementation is not safe for concurrent
// use.
func (pq *PriorityQueue[T]) Lock() {}

// Unlock is a no-op provided for API compatibility with a thread-safe
// priority queue twin.  This implementation is not safe for concurrent
// use.
func (pq *PriorityQueue[T]) Unlock() {}

// All returns an iterator that yields the elements of the queue in
// priority order, minimum element first.
//
// The iteration is non-destructive: it drains a private copy of the heap
// (a snapshot taken when All is called, built with the queue's own
// ordering), so the queue is unchanged afterwards and elements inserted
// or removed after the call do not affect the sequence.  Mutating the
// queue from inside the loop is safe.
// Complexity is O(n log n) to build the snapshot, then O(log n) per
// element as it drains.
//
//	for item := range pq.All() {
//		fmt.Println(item)
//	}
func (pq *PriorityQueue[T]) All() iter.Seq[T] {
	if pq == nil || pq.h == nil {
		return func(func(T) bool) {} // a nil queue iterates as an empty one
	}
	snapshot := heap.NewHeapFunc(pq.cmp)
	for i := 0; i < pq.h.Len(); i++ {
		v, _ := pq.h.GetValue(i)
		snapshot.Push(v)
	}
	return func(yield func(T) bool) {
		for {
			v, found := snapshot.Pop()
			if !found {
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}
