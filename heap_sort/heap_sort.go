/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package heap_sort implements heap sort on top of the generic min-heap
// in charon/heap.  Elements are added with Insert or InsertArray and
// extracted in ascending order with Sort or descending order with
// SortDown.  The two Sort calls drain the sorter; it is reusable
// afterwards.
//
// Elements are stored and returned by value — the result of
// Sort/SortDown is a []T of copies, not a slice of pointers into the
// sorter.
//
// The element type never implements an interface: sorters of naturally
// ordered types (all integers, floats and strings) are created with
// NewHeapSort, which orders by the built-in < and > operators; sorters
// of any other type — including structs ordered by one field — are
// created with NewHeapSortFunc, which takes a caller supplied
// comparison function.
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
// The sorter is NOT safe for concurrent use; its thread-safe twin is
// github.com/pschlump/charon/heap_sort_ts.
package heap_sort

import (
	"cmp"

	"github.com/pschlump/charon/heap"
)

// HeapSort collects the elements to sort in a charon/heap min-heap and
// drains them in order.  Use NewHeapSort for naturally ordered element
// types (numbers, strings) or NewHeapSortFunc for a caller supplied
// comparison function.  The zero value is an empty sorter for reads,
// but Insert on it panics — create sorters with the constructors.
type HeapSort[T any] struct {
	h *heap.Heap[T]
}

// NewHeapSort creates a new, empty sorter for any naturally ordered
// element type (all integers, floats and strings — cmp.Ordered).
// Ordering uses the built-in < and > operators of T; no interface and
// no boxing is involved.
// Complexity is O(1).
func NewHeapSort[T cmp.Ordered]() *HeapSort[T] {
	return &HeapSort[T]{h: heap.NewHeap[T]()}
}

// NewHeapSortFunc creates a new, empty sorter that orders elements with
// the caller supplied comparison function fx — for example a struct
// ordered by one of its fields.  A reversed comparison makes Sort
// return descending order and SortDown ascending order.
// It panics if fx is nil.
// Complexity is O(1).
func NewHeapSortFunc[T any](fx func(a, b T) int) *HeapSort[T] {
	if fx == nil {
		panic("heap_sort: NewHeapSortFunc called with a nil comparison function")
	}
	return &HeapSort[T]{h: heap.NewHeapFunc(fx)}
}

// Insert adds a single element to the set of values to be sorted.
// It panics on a nil sorter or on a zero-value sorter (no underlying
// heap); these are the only panics on non-constructor calls in the
// package (together with InsertArray) — every other operation treats
// both as an empty sorter.
// Complexity is O(log n).
func (hs *HeapSort[T]) Insert(n T) {
	if hs == nil || hs.h == nil {
		panic("heap_sort: Insert called on a nil or zero-value sorter (create the sorter with NewHeapSort or NewHeapSortFunc)")
	}
	hs.h.Push(n)
}

// InsertArray adds a slice of elements to the values to be sorted and
// rebuilds the heap in one bottom-up pass — cheaper than Insert per
// element.  It may be called repeatedly; each call re-heapifies the
// combined data, so mixing Insert and InsertArray is fine.
// It panics on a nil sorter or on a zero-value sorter (no underlying
// heap).
// Complexity is O(m + n) where m = len(batch) and n = the sorter's
// length after the append.
func (hs *HeapSort[T]) InsertArray(batch []T) {
	if hs == nil || hs.h == nil {
		panic("heap_sort: InsertArray called on a nil or zero-value sorter (create the sorter with NewHeapSort or NewHeapSortFunc)")
	}
	hs.h.AppendHeap(batch)
	n := hs.h.Len()
	for i := n/2 - 1; i >= 0; i-- {
		hs.h.Heapify(n, i)
	}
}

// Sort removes all of the elements and returns them as a slice sorted
// in ascending order.  The sorter is empty (and reusable) after this
// call; Sort on an empty sorter returns an empty slice.
// Complexity is O(n log n).
func (hs *HeapSort[T]) Sort() (rv []T) {
	if hs == nil || hs.h == nil {
		return []T{}
	}
	n := hs.h.Len()
	rv = make([]T, 0, n)
	for range n {
		v, _ := hs.h.Pop()
		rv = append(rv, v)
	}
	return
}

// SortDown removes all of the elements and returns them as a slice
// sorted in descending order.  The sorter is empty (and reusable) after
// this call; SortDown on an empty sorter returns an empty slice.
// Complexity is O(n log n).
func (hs *HeapSort[T]) SortDown() (rv []T) {
	if hs == nil || hs.h == nil {
		return []T{}
	}
	n := hs.h.Len()
	rv = make([]T, n)
	for i := n - 1; i >= 0; i-- {
		v, _ := hs.h.Pop()
		rv[i] = v
	}
	return
}

// Len will return the number of items waiting to be sorted.
// Complexity is O(1).
func (hs *HeapSort[T]) Len() int {
	if hs == nil || hs.h == nil {
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
	if hs == nil || hs.h == nil {
		return
	}
	hs.h.Truncate()
}

// Lock is a no-op provided for API compatibility with a thread-safe
// heap_sort twin.  This implementation is not safe for concurrent use.
func (hs *HeapSort[T]) Lock() {}

// Unlock is a no-op provided for API compatibility with a thread-safe
// heap_sort twin.  This implementation is not safe for concurrent use.
func (hs *HeapSort[T]) Unlock() {}
