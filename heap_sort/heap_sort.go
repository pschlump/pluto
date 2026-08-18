// Copyright (C) 2021 Philip Schlump. All rights reserved.

// Package heap_sort provides a sorting facility built on top of the generic
// min-heap in github.com/pschlump/pluto/heap.  Elements (any type
// implementing comparable.Comparable) are inserted with Insert or
// InsertArray and extracted in ascending order with Sort or descending
// order with SortDown.  Sorting drains the underlying heap.
package heap_sort

/*
= Heap Sort

1. Sort
2. SortDown
2. Insert
*/

import (
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap"
)

type heap_sort[T comparable.Comparable] struct {
	theHeap *heap.Heap[T]
}

// NewHeapSort creates a new heap_sort and returns it.
// Complexity is O(1).
func NewHeapSort[T comparable.Comparable]() (rv *heap_sort[T]) {
	rv = &heap_sort[T]{
		theHeap: heap.NewHeap[T](),
	}
	return
}

// Insert adds a single element to the set of values to be sorted.
// Complexity is O(log n).
func (srt *heap_sort[T]) Insert(n *T) {
	srt.theHeap.Push(n)
}

// InsertArray adds a set of elements to the values to be sorted.
// Complexity is O(m + n) where m = len(n); the heap is rebuilt in O(n).
func (srt *heap_sort[T]) InsertArray(n []*T) {
	srt.theHeap.AppendHeap(n)
	for i := srt.theHeap.Len()/2 - 1; i >= 0; i-- {
		srt.theHeap.Heapify(srt.theHeap.Len(), i)
	}
}

// Sort removes all of the elements and returns them as a slice sorted in
// ascending order.  The heap_sort is empty after this call.
// Complexity is O(n log n).
func (srt *heap_sort[T]) Sort() (rv []*T) {
	n := srt.theHeap.Len()
	rv = make([]*T, 0, n)
	for i := 0; i < n; i++ {
		rv = append(rv, srt.theHeap.Pop())
	}
	return
}

// SortDown removes all of the elements and returns them as a slice sorted in
// descending order.  The heap_sort is empty after this call.
// Complexity is O(n log n).
func (srt *heap_sort[T]) SortDown() (rv []*T) {
	n := srt.theHeap.Len()
	rv = make([]*T, n)
	for i, j := 0, n; i < n; i++ {
		j--
		rv[j] = srt.theHeap.Pop()
	}
	return
}

// Len will return the number of items in the heap.
// Complexity is O(1).
func (srt *heap_sort[T]) Len() int {
	return srt.theHeap.Len()
}

// Length will return the number of items in the heap.  It is an alias for Len.
// Complexity is O(1).
func (srt *heap_sort[T]) Length() int {
	return srt.theHeap.Len()
}

// Truncate removes all data from the heap.
// Complexity is O(1).
func (srt *heap_sort[T]) Truncate() {
	srt.theHeap.Truncate()
}
