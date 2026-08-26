/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package priority_queue_ts implements a generic priority queue that is
// safe for concurrent use, backed by the thread-safe binary min-heap in
// charon/heap_ts.  It is the thread-safe twin of
// github.com/pschlump/charon/priority_queue — the same API — with the
// addition of the Lock/Unlock pair and the Nl-prefixed (no-lock) methods
// for compound operations.
//
// It is the charon rework of github.com/pschlump/pluto/priority_queue_ts:
// the comparable.Comparable interface constraint is replaced with plain
// Go type parameters, elements are stored and returned by value, and the
// not-found and out-of-range outcomes are reported with false returns
// instead of nil pointers and errors.
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
// Concurrency model:
//
//	Every operation delegates to the guarded methods of the underlying
//	heap_ts.Heap (reads under its RLock, writes under its write lock), so
//	reads run in parallel with each other and writes are exclusive.
//	All operates on a snapshot taken when it is called (copied out under
//	the heap's write lock, then drained privately), so it is safe to use
//	concurrently with any queue operation — including mutating the queue
//	from inside the loop — and never observes later modifications.
//	Positions returned by Search may be invalidated by any concurrent
//	mutation of the queue; use Lock/Unlock with the Nl-prefixed methods
//	for atomic search-then-update sequences.
//	Lock takes the queue's write lock; while it is held only the
//	Nl-prefixed methods may be called — calling a regular method will
//	deadlock.
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
// Run the tests with -race.
package priority_queue_ts

import (
	"cmp"
	"iter"

	"github.com/pschlump/charon/heap_ts"
)

// PriorityQueue is a thread-safe priority queue of T values ordered by
// its comparison function.  Use NewPriorityQueue for naturally ordered
// element types (numbers, strings) or NewPriorityQueueFunc for a caller
// supplied comparison function.  The zero value is an empty queue for
// reads, but Insert on it panics — create queues with the constructors.
type PriorityQueue[T any] struct {
	h *heap_ts.Heap[T]

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
	return newPQ(heap_ts.Compare[T])
}

// NewPriorityQueueFunc creates a new, empty priority queue that orders
// elements with the caller supplied comparison function fx — for example
// a struct ordered by its priority field.
// It panics if fx is nil.
// Complexity is O(1).
func NewPriorityQueueFunc[T any](fx func(a, b T) int) *PriorityQueue[T] {
	if fx == nil {
		panic("priority_queue_ts: NewPriorityQueueFunc called with a nil comparison function")
	}
	return newPQ(fx)
}

func newPQ[T any](cmp func(a, b T) int) *PriorityQueue[T] {
	return &PriorityQueue[T]{h: heap_ts.NewHeapFunc(cmp), cmp: cmp}
}

// ok reports whether the queue has an underlying heap to delegate to.
func (pq *PriorityQueue[T]) ok() bool {
	return pq != nil && pq.h != nil
}

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Len() int {
	if !pq.ok() {
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
// The element is returned by value: it is an independent copy taken
// under the heap's read lock.  The element may of course be popped by
// another goroutine as soon as Peek returns.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Peek() (rv T, found bool) {
	if !pq.ok() {
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
	if !pq.ok() {
		panic("priority_queue_ts: Insert called on a nil or zero-value queue (create the queue with NewPriorityQueue or NewPriorityQueueFunc)")
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
	if !pq.ok() {
		return
	}
	return pq.h.Pop()
}

// Search finds the first element that compares equal to cmpVal and
// returns it along with its position in the underlying heap (usable with
// UpdatePriority and Delete), or (zero, -1, false) if no element matches.
// The probe only needs the fields that the comparison function reads.
//
// Note: the returned position may be invalidated by any concurrent
// mutation of the queue; use Lock/Unlock with the Nl-prefixed methods
// for atomic search-then-update sequences.
// Complexity is O(n).
func (pq *PriorityQueue[T]) Search(cmpVal T) (rv T, pos int, found bool) {
	if !pq.ok() {
		return
	}
	return pq.h.Search(cmpVal)
}

// UpdatePriority replaces the element at position pos (a position
// previously returned by Search) with newVal and re-establishes the heap
// ordering.  It reports false and does nothing if pos is out of range.
// Replacing in place is cheaper than a Delete followed by an Insert.
//
// As with Search, pos may have been invalidated by a concurrent mutation
// between obtaining it and calling UpdatePriority; use Lock/Unlock with
// the Nl-prefixed methods for atomic sequences.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) UpdatePriority(pos int, newVal T) bool {
	if !pq.ok() {
		return false
	}
	return pq.h.Fix(pos, newVal)
}

// Delete removes and returns the element at position pos (a position
// previously returned by Search) from the queue.  It reports false if pos
// is out of range.  As with UpdatePriority, pos may have been
// invalidated by a concurrent mutation.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Delete(pos int) (rv T, found bool) {
	if !pq.ok() {
		return
	}
	return pq.h.Delete(pos)
}

// Truncate removes all data from the queue.  The comparison function is
// kept, so the queue remains usable and can simply be refilled.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Truncate() {
	if !pq.ok() {
		return
	}
	pq.h.Truncate()
}

// -------------------------------------------------------------------------------------------------------
// Exposed write lock and the Nl (no-lock) methods for compound
// operations.  While the lock is held only call the Nl-prefixed methods;
// calling a regular method will deadlock.
// -------------------------------------------------------------------------------------------------------

// Lock takes the queue's write lock, allowing a group of operations to
// be performed atomically (e.g. a Search-then-UpdatePriority sequence).
// While the lock is held only call the Nl-prefixed (no-lock) methods;
// calling a regular method will deadlock.  Pair every Lock with a
// corresponding Unlock.  Locking a nil queue is a no-op.
func (pq *PriorityQueue[T]) Lock() {
	if !pq.ok() {
		return
	}
	pq.h.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil queue
// is a no-op.
func (pq *PriorityQueue[T]) Unlock() {
	if !pq.ok() {
		return
	}
	pq.h.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlLen() int {
	return pq.h.NlLen()
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlIsEmpty() bool {
	return pq.NlLen() == 0
}

// NlPeek is Peek without locking; call it only while holding Lock.  It
// reports false on an empty queue.
func (pq *PriorityQueue[T]) NlPeek() (rv T, found bool) {
	return pq.h.NlGetValue(0)
}

// NlInsert is Insert without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlInsert(n T) {
	pq.h.NlPush(n)
}

// NlPop is Pop without locking; call it only while holding Lock.  It
// reports false on an empty queue.
func (pq *PriorityQueue[T]) NlPop() (rv T, found bool) {
	return pq.h.NlPop()
}

// NlSearch is Search without locking; call it only while holding Lock.
// It reports (zero, -1, false) when no element matches.
func (pq *PriorityQueue[T]) NlSearch(cmpVal T) (rv T, pos int, found bool) {
	pos = -1
	for ii := 0; ii < pq.NlLen(); ii++ {
		v, ok := pq.h.NlGetValue(ii)
		if ok && pq.cmp(v, cmpVal) == 0 {
			return v, ii, true
		}
	}
	return
}

// NlUpdatePriority is UpdatePriority without locking; call it only while
// holding Lock.  It reports false if pos is out of range.
func (pq *PriorityQueue[T]) NlUpdatePriority(pos int, newVal T) bool {
	return pq.h.NlFix(pos, newVal)
}

// NlDelete is Delete without locking; call it only while holding Lock.
// It reports false if pos is out of range.
func (pq *PriorityQueue[T]) NlDelete(pos int) (rv T, found bool) {
	return pq.h.NlDelete(pos)
}

// -------------------------------------------------------------------------------------------------------
// Iterator
// -------------------------------------------------------------------------------------------------------

// All returns an iterator that yields the elements of the queue in
// priority order, minimum element first.
//
// The iteration is non-destructive: it drains a private copy of the heap
// (a snapshot copied out under the heap's write lock when All is called,
// built with the queue's own ordering), so the queue is unchanged
// afterwards, it is safe to call queue methods — including from inside
// the loop — and elements inserted or removed after the call do not
// affect the sequence.
// Complexity is O(n log n) to build the snapshot, then O(log n) per
// element as it drains.
//
//	for item := range pq.All() {
//		fmt.Println(item)
//	}
func (pq *PriorityQueue[T]) All() iter.Seq[T] {
	if !pq.ok() {
		return func(func(T) bool) {} // a nil queue iterates as an empty one
	}
	snapshot := heap_ts.NewHeapFunc(pq.cmp)
	pq.h.Lock()
	for i := 0; i < pq.h.NlLen(); i++ {
		v, found := pq.h.NlGetValue(i)
		if !found {
			break
		}
		snapshot.NlPush(v)
	}
	pq.h.Unlock()
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
