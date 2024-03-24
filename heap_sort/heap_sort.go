package heap_sort

/*
= Heap Sort

1. Sort
2. SortDown
2. Insert
*/

import (
	// "fmt"

	// "github.com/pschlump/dbgo"
	// "github.com/pschlump/MiscLib"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap"
)

type heap_sort[T comparable.Comparable] struct {
	theHeap *heap.Heap[T]
}

// Create a new heap_sort and return it.
// Complexity is O(1).
func NewHeapSort[T comparable.Comparable]() (rv *heap_sort[T]) {
	rv = &heap_sort[T]{
		theHeap: heap.NewHeap[T](),
	}
	return
}

// Complexity O(n log n)
func (srt *heap_sort[T]) Insert(n *T) {
	srt.theHeap.Push(n)
}

// Complexity O(n log n)
func (srt *heap_sort[T]) InsertArray(n []*T) {
	//for _, v := range n {
	//	srt.theHeap.Push(v)
	//}
	srt.theHeap.AppendHeap(n)
	srt.theHeap.Heapify(srt.theHeap.Len(), 0)
}

// Complexity O(n log n)
func (srt *heap_sort[T]) Sort() (rv []*T) {
	n := srt.theHeap.Len()
	rv = make([]*T, 0, n)
	for i := 0; i < n; i++ {
		rv = append(rv, srt.theHeap.Pop())
	}
	return
}

// Complexity O(n log n)
func (srt *heap_sort[T]) SortDown() (rv []*T) {
	n := srt.theHeap.Len()
	rv = make([]*T, n, n)
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
func (srt *heap_sort[T]) Length() int {
	return srt.theHeap.Len()
}

// Truncate removes all data from the heap.
// Complexity is O(1).
func (srt *heap_sort[T]) Truncate() {
	srt.theHeap = heap.NewHeap[T]()
}
