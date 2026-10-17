/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package stable_pq implements a stable min-priority queue (Sedgwick's
// StableMinPQ, Algorithms 4th ed. §2.4 exercise): a minimum priority
// queue in which elements with equal keys come out in the order they
// were inserted — first in, first out among ties.
//
// Stability is achieved the way algs4 does it: every inserted element
// receives a strictly increasing timestamp, and the heap orders by the
// (key, timestamp) pair — a tie on the key falls through to the
// timestamp, which is a total order with no ties.  The heap itself is
// pluto's own binary heap over that pair (an intra-pluto composition,
// like priority_queue and huffman's merge).
//
// The queue supports the following operations:
//
//	Peek     — look at the minimum element (ties: oldest).			O(1)
//	Insert   — add an element.						O(log n)
//	Pop      — remove and return the minimum element (ties: oldest).	O(log n)
//	All      — iterate in (key, insertion) order, non-destructive.		O(n log n)
//	Len / Length / IsEmpty / Truncate — size and reset.			O(1)
//
// The element type never implements an interface: queues of naturally
// ordered types (all integers, floats and strings) are created with
// NewStablePQ, which orders by the built-in < and > operators; queues
// of any other type — including structs ordered by a priority field —
// are created with NewStablePQFunc, which takes a caller supplied
// comparison function.
//
// A nil *StablePQ and the zero value both behave as an empty queue for
// every operation except Insert, which panics with a message naming the
// constructors.  The package panics in exactly three situations, all
// programmer errors:
//
//	NewStablePQFunc(nil)         — nil comparison function, caught at construction.
//	Insert on a nil queue        — a nil queue cannot store an element.
//	Insert on a zero-value queue — no underlying heap; the message names the constructors.
//
// The queue is NOT safe for concurrent use; wrap it in a mutex (there
// is deliberately no _ts twin — priority_queue_ts covers the callers
// that do not need stability).
package stable_pq

import (
	"cmp"
	"iter"

	"github.com/pschlump/pluto/heap"
)

// spqItem is what the heap actually stores: the caller's key paired
// with the strictly increasing insertion stamp that breaks ties.  It is
// unexported — the queue's public surface is bare T.
type spqItem[T any] struct {
	key T
	seq int64
}

// StablePQ is a stable min-priority queue of T values: the minimum
// element comes out first, and among elements that compare equal the
// oldest comes out first.  Use NewStablePQ for naturally ordered
// element types (numbers, strings) or NewStablePQFunc for a caller
// supplied comparison function.  The zero value is an empty queue for
// reads, but Insert on it panics — create queues with the constructors.
type StablePQ[T any] struct {
	// h is the underlying heap over (key, seq) pairs.  It is nil on a
	// zero-value queue, which every read treats as empty.
	h *heap.Heap[spqItem[T]]

	// cmp is the caller's key comparison (a copy kept beside the heap,
	// the priority_queue precedent) — needed to rebuild the item
	// comparator when All copies the heap.
	cmp func(a, b T) int

	// stamp is the timestamp of the most recent Insert; the next Insert
	// uses seq = stamp+1 (so the first element is 1, like algs4).  It is
	// never reset — monotonic across the queue's whole life, which keeps
	// the "older is smaller" argument true even across a Truncate and
	// refill.
	stamp int64
}

// NewStablePQ creates a new, empty stable min-priority queue for any
// naturally ordered element type (all integers, floats and strings —
// cmp.Ordered).  Ordering uses the built-in < and > operators of T.
// Complexity is O(1).
func NewStablePQ[T cmp.Ordered]() *StablePQ[T] {
	return newStablePQ[T](cmp.Compare[T])
}

// NewStablePQFunc creates a new, empty stable min-priority queue that
// orders keys with the caller supplied comparison function fx — for
// example a struct ordered by its priority field.
// It panics if fx is nil.
// Complexity is O(1).
func NewStablePQFunc[T any](fx func(a, b T) int) *StablePQ[T] {
	if fx == nil {
		panic("stable_pq: NewStablePQFunc called with a nil comparison function")
	}
	return newStablePQ[T](fx)
}

func newStablePQ[T any](cmp func(a, b T) int) *StablePQ[T] {
	return &StablePQ[T]{h: heap.NewHeapFunc(itemCmp[T](cmp)), cmp: cmp}
}

// itemCmp builds the (key, seq) lexicographic comparator the heap
// stores: keys first, then the insertion stamp — smaller (older) stamp
// wins a tie, which is exactly FIFO among equal keys.  Any total order
// on ties makes the heap correct; this one makes it stable.
func itemCmp[T any](cmp func(a, b T) int) func(a, b spqItem[T]) int {
	return func(a, b spqItem[T]) int {
		if c := cmp(a.key, b.key); c != 0 {
			return c
		}
		switch {
		case a.seq < b.seq:
			return -1
		case a.seq > b.seq:
			return 1
		}
		return 0
	}
}

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (pq *StablePQ[T]) Len() int {
	if pq == nil || pq.h == nil {
		return 0
	}
	return pq.h.Len()
}

// Length is an alias for Len; it returns the number of elements in the
// queue.
// Complexity is O(1).
func (pq *StablePQ[T]) Length() int {
	return pq.Len()
}

// IsEmpty returns true if the queue has no elements.
// Complexity is O(1).
func (pq *StablePQ[T]) IsEmpty() bool {
	return pq.Len() == 0
}

// Peek returns the minimum element in the queue without removing it —
// among elements that compare equal, the one inserted earliest — or
// false if the queue is empty.
//
// The element is returned by value; it does not alias the queue's
// internals.
// Complexity is O(1).
func (pq *StablePQ[T]) Peek() (rv T, found bool) {
	if pq == nil || pq.h == nil {
		return
	}
	item, found := pq.h.Peek()
	return item.key, found
}

// Insert adds the element n to the queue, stamping it with the next
// insertion sequence number.
// It panics on a nil queue or on a zero-value queue (no underlying
// heap); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty queue.
// Complexity is O(log n).
func (pq *StablePQ[T]) Insert(n T) {
	if pq == nil || pq.h == nil {
		panic("stable_pq: Insert called on a nil or zero-value queue (create the queue with NewStablePQ or NewStablePQFunc)")
	}
	pq.stamp++
	pq.h.Push(spqItem[T]{key: n, seq: pq.stamp})
}

// Pop removes and returns the minimum element in the queue — among
// elements that compare equal, the one inserted earliest — or false if
// the queue is empty.
//
// The element is returned by value; the queue no longer holds any
// reference to it.
// Complexity is O(log n).
func (pq *StablePQ[T]) Pop() (rv T, found bool) {
	if pq == nil || pq.h == nil {
		return
	}
	item, found := pq.h.Pop()
	return item.key, found
}

// Truncate removes all data from the queue.  The comparison function is
// kept (and the stamp keeps counting up, never resetting), so the queue
// remains usable and can simply be refilled.
// Complexity is O(1).
func (pq *StablePQ[T]) Truncate() {
	if pq == nil || pq.h == nil {
		return
	}
	pq.h.Truncate()
}

// Lock is a no-op provided for API compatibility with the thread-safe
// priority queue twins.  This implementation is not safe for concurrent
// use.
func (pq *StablePQ[T]) Lock() {}

// Unlock is a no-op provided for API compatibility with the thread-safe
// priority queue twins.  This implementation is not safe for concurrent
// use.
func (pq *StablePQ[T]) Unlock() {}

// All returns an iterator that yields the elements of the queue in
// priority order — minimum key first, and among equal keys in insertion
// order, the exact order Pop would produce.
//
// The iteration is non-destructive: it drains a private copy of the
// heap (a snapshot taken when All is called), so the queue is unchanged
// afterwards and elements inserted or removed after the call do not
// affect the sequence.  Mutating the queue from inside the loop is
// safe.
// Complexity is O(n log n) to build the snapshot, then O(log n) per
// element as it drains.
//
//	for item := range pq.All() {
//		fmt.Println(item)
//	}
func (pq *StablePQ[T]) All() iter.Seq[T] {
	if pq == nil || pq.h == nil {
		return func(func(T) bool) {} // a nil queue iterates as an empty one
	}
	snapshot := heap.NewHeapFunc(itemCmp(pq.cmp))
	for i := 0; i < pq.h.Len(); i++ {
		v, _ := pq.h.GetValue(i)
		snapshot.Push(v)
	}
	return func(yield func(T) bool) {
		for {
			item, found := snapshot.Pop()
			if !found {
				return
			}
			if !yield(item.key) {
				return
			}
		}
	}
}
