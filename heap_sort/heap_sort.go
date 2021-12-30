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
	rv = &heap_sort[T]{
		theHeap: heap.NewHeap[T](),
	}
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
	for i := 0; i < n; i++ {
		rv = append ( rv, pq.theHeap.Pop() )
	}
	return
}

// Complexity O(n log n)
func (pq *heap_sort[T]) SortDown() ( rv []*T ) {
	n := pq.theHeap.Len()
	rv = make ( []*T, n, n )
	for i, j := 0, n; i < n; i++ {
		j--
		rv[j] = pq.theHeap.Pop() 
	}
	return
}

// xyzzy TODO - add Len(), Length()
