package heap_sort

/*
= Heap Sort

1. Sort
2. SortDown
2. Insert
*/

import (
	// "fmt"

	// "github.com/pschlump/godebug"
	// "github.com/pschlump/MiscLib"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap"
)

type heap_sort[T comparable.Comparable] struct {
	theHeap *heap.Heap[T] 
}

// Create a new heap_sort and return it.
// Complexity is O(1).
func NewHeapSort[T comparable.Comparable] () ( rv *heap_sort[T] ) {
	// We don't have to "heapify" at this point becasue we start all heaps with an empty set of data.
	rv.theHeap = heap.NewHeap[T]()
	return 
}

// Complexity O(n log n)
func (pq *heap_sort[T])Insert(n *T) {
	pq.theHeap.Push(n)
}

// Complexity O(n log n)
func (pq *heap_sort[T]) Sort() ( rv []*T ) {
	n := pq.theHeap.Len()
	rv = make ( []*T, 0, n )
	for i := 0; i < pq.theHeap.Len(); i++ {
		x := pq.theHeap.Pop() 
		rv = append ( rv, x )
	}
	return
}

// Complexity O(n log n)
func (pq *heap_sort[T]) SortDown() ( rv []*T ) {
	n := pq.theHeap.Len()
	rv = make ( []*T, n, n )
	j := n-1
	for i := 0; i < pq.theHeap.Len(); i++ {
		x := pq.theHeap.Pop() 
		rv[j] = x
		j--
	}
	return
}

