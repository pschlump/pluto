/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package heap_sort_ts implements a heap sort that is safe for
// concurrent use, backed by the thread-safe min-heap in pluto/heap_ts.
// It is the thread-safe twin of github.com/pschlump/pluto/heap_sort —
// the same API — with the addition of the Lock/Unlock pair and the
// Nl-prefixed (no-lock) methods for compound operations.
//
// Elements are added with Insert or InsertArray and extracted in
// ascending order with Sort or descending order with SortDown.  The two
// Sort calls drain the sorter; it is reusable afterwards.  Elements are
// stored and returned by value — the result of Sort/SortDown is a []T of
// copies, not a slice of pointers into the sorter.
//
// The element type never implements an interface: sorters of naturally
// ordered types (all integers, floats and strings) are created with
// NewHeapSort, which orders by the built-in < and > operators; sorters
// of any other type — including structs ordered by one field — are
// created with NewHeapSortFunc, which takes a caller supplied comparison
// function.
//
// Concurrency model:
//
//	Every operation delegates to the guarded methods of the underlying
//	heap_ts.Heap (reads under its RLock, writes under its write lock), so
//	reads run in parallel with each other and writes are exclusive.
//	The three multi-step operations — InsertArray (append plus rebuild)
//	and Sort/SortDown (drain) — hold the heap's write lock for the whole
//	operation, so each is atomic: a concurrent Insert can never observe
//	the heap mid-rebuild, and Sort returns every element present when it
//	started, leaving the sorter empty.
//	Lock takes the sorter's write lock; while it is held only the
//	Nl-prefixed methods may be called — calling a regular method will
//	deadlock.
//
// A nil *HeapSort and the zero value both behave as an empty sorter for
// every operation except the two inserts, which panic with a message
// naming the constructors.  The package panics in exactly three
// situations, all programmer errors:
//
//	NewHeapSortFunc(nil)          — nil comparison function, caught at construction.
//	Insert/InsertArray on nil     — a nil sorter cannot store an element.
//	Insert/InsertArray on zero    — no underlying heap; the message names the constructors.
//
// Run the tests with -race.
package heap_sort_ts

import (
	"cmp"

	"github.com/pschlump/pluto/heap_ts"
)

// HeapSort collects the elements to sort in a thread-safe
// heap_ts.Heap and drains them in order.  Use NewHeapSort for naturally
// ordered element types (numbers, strings) or NewHeapSortFunc for a
// caller supplied comparison function.  The zero value is an empty
// sorter for reads, but Insert on it panics — create sorters with the
// constructors.
type HeapSort[T any] struct {
	h *heap_ts.Heap[T]
}

// NewHeapSort creates a new, empty sorter for any naturally ordered
// element type (all integers, floats and strings — cmp.Ordered).
// Ordering uses the built-in < and > operators of T; no interface and
// no boxing is involved.
// Complexity is O(1).
func NewHeapSort[T cmp.Ordered]() *HeapSort[T] {
	return &HeapSort[T]{h: heap_ts.NewHeap[T]()}
}

// NewHeapSortFunc creates a new, empty sorter that orders elements with
// the caller supplied comparison function fx — for example a struct
// ordered by one of its fields.  A reversed comparison makes Sort
// return descending order and SortDown ascending order.
// It panics if fx is nil.
// Complexity is O(1).
func NewHeapSortFunc[T any](fx func(a, b T) int) *HeapSort[T] {
	if fx == nil {
		panic("heap_sort_ts: NewHeapSortFunc called with a nil comparison function")
	}
	return &HeapSort[T]{h: heap_ts.NewHeapFunc(fx)}
}

// ok reports whether the sorter has an underlying heap to delegate to.
func (hs *HeapSort[T]) ok() bool {
	return hs != nil && hs.h != nil
}

// Insert adds a single element to the set of values to be sorted.
// It panics on a nil sorter or on a zero-value sorter (no underlying
// heap); these are the only panics on non-constructor calls in the
// package (together with InsertArray) — every other operation treats
// both as an empty sorter.
// Complexity is O(log n).
func (hs *HeapSort[T]) Insert(n T) {
	if !hs.ok() {
		panic("heap_sort_ts: Insert called on a nil or zero-value sorter (create the sorter with NewHeapSort or NewHeapSortFunc)")
	}
	hs.h.Push(n)
}

// InsertArray adds a slice of elements to the values to be sorted and
// rebuilds the heap in one bottom-up pass — cheaper than Insert per
// element.  It may be called repeatedly; each call re-heapifies the
// combined data, so mixing Insert and InsertArray is fine.  The batch is
// copied into the sorter, so the caller's slice can be mutated
// afterwards.
//
// The append and the rebuild run under one hold of the heap's write
// lock, so the whole batch lands atomically: a concurrent operation can
// never observe the heap mid-rebuild.
// It panics on a nil sorter or on a zero-value sorter (no underlying
// heap).
// Complexity is O(m + n) where m = len(batch) and n = the sorter's
// length after the append.
func (hs *HeapSort[T]) InsertArray(batch []T) {
	if !hs.ok() {
		panic("heap_sort_ts: InsertArray called on a nil or zero-value sorter (create the sorter with NewHeapSort or NewHeapSortFunc)")
	}
	hs.h.Lock()
	defer hs.h.Unlock()
	hs.h.NlAppendHeap(batch)
	n := hs.h.NlLen()
	for i := n/2 - 1; i >= 0; i-- {
		hs.h.NlHeapify(n, i)
	}
}

// Sort removes all of the elements and returns them as a slice sorted
// in ascending order.  The sorter is empty (and reusable) after this
// call; Sort on an empty sorter returns an empty slice.
//
// The whole drain runs under one hold of the heap's write lock, so it
// is atomic: the result contains exactly the elements that were in the
// sorter when Sort started, and the sorter is empty the moment Sort
// returns.
// Complexity is O(n log n).
func (hs *HeapSort[T]) Sort() (rv []T) {
	if !hs.ok() {
		return []T{}
	}
	hs.h.Lock()
	defer hs.h.Unlock()
	n := hs.h.NlLen()
	rv = make([]T, 0, n)
	for range n {
		v, _ := hs.h.NlPop()
		rv = append(rv, v)
	}
	return
}

// SortDown removes all of the elements and returns them as a slice
// sorted in descending order.  The sorter is empty (and reusable) after
// this call; SortDown on an empty sorter returns an empty slice.
// Like Sort, the whole drain is atomic under one hold of the write lock.
// Complexity is O(n log n).
func (hs *HeapSort[T]) SortDown() (rv []T) {
	if !hs.ok() {
		return []T{}
	}
	hs.h.Lock()
	defer hs.h.Unlock()
	n := hs.h.NlLen()
	rv = make([]T, n)
	for i := n - 1; i >= 0; i-- {
		v, _ := hs.h.NlPop()
		rv[i] = v
	}
	return
}

// Len will return the number of items waiting to be sorted.
// Complexity is O(1).
func (hs *HeapSort[T]) Len() int {
	if !hs.ok() {
		return 0
	}
	return hs.h.Len()
}

// Length will return the number of items waiting to be sorted.  It is
// an alias for Len.
// Complexity is O(1).
func (hs *HeapSort[T]) Length() int {
	return hs.Len()
}

// IsEmpty returns true if the sorter has no elements.
// Complexity is O(1).
func (hs *HeapSort[T]) IsEmpty() bool {
	return hs.Len() == 0
}

// Truncate removes all data from the sorter, releasing the underlying
// storage so the GC can reclaim it.
// Complexity is O(1).
func (hs *HeapSort[T]) Truncate() {
	if !hs.ok() {
		return
	}
	hs.h.Truncate()
}

// -------------------------------------------------------------------------------------------------------
// Exposed write lock and the Nl (no-lock) methods for compound
// operations.  While the lock is held only call the Nl-prefixed methods;
// calling a regular method will deadlock.
// -------------------------------------------------------------------------------------------------------

// Lock takes the sorter's write lock, allowing a group of operations to
// be performed atomically (e.g. an insert-batch-then-sort sequence).
// While the lock is held only call the Nl-prefixed (no-lock) methods;
// calling a regular method will deadlock.  Pair every Lock with a
// corresponding Unlock.  Locking a nil sorter is a no-op.
func (hs *HeapSort[T]) Lock() {
	if !hs.ok() {
		return
	}
	hs.h.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil sorter
// is a no-op.
func (hs *HeapSort[T]) Unlock() {
	if !hs.ok() {
		return
	}
	hs.h.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (hs *HeapSort[T]) NlLen() int {
	return hs.h.NlLen()
}

// NlIsEmpty is IsEmpty without locking; call it only while holding
// Lock.
func (hs *HeapSort[T]) NlIsEmpty() bool {
	return hs.NlLen() == 0
}

// NlInsert is Insert without locking; call it only while holding Lock.
func (hs *HeapSort[T]) NlInsert(n T) {
	hs.h.NlPush(n)
}

// NlInsertArray is InsertArray without locking; call it only while
// holding Lock.
func (hs *HeapSort[T]) NlInsertArray(batch []T) {
	hs.h.NlAppendHeap(batch)
	n := hs.h.NlLen()
	for i := n/2 - 1; i >= 0; i-- {
		hs.h.NlHeapify(n, i)
	}
}

// NlSort is Sort without locking; call it only while holding Lock.  It
// returns an empty slice on an empty sorter.
func (hs *HeapSort[T]) NlSort() (rv []T) {
	n := hs.h.NlLen()
	rv = make([]T, 0, n)
	for range n {
		v, _ := hs.h.NlPop()
		rv = append(rv, v)
	}
	return
}

// NlSortDown is SortDown without locking; call it only while holding
// Lock.  It returns an empty slice on an empty sorter.
func (hs *HeapSort[T]) NlSortDown() (rv []T) {
	n := hs.h.NlLen()
	rv = make([]T, n)
	for i := n - 1; i >= 0; i-- {
		v, _ := hs.h.NlPop()
		rv[i] = v
	}
	return
}
