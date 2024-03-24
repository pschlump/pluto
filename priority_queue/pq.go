package priority_queue

/*
= Priority Queue Operations

1. Peek
2. Insert
3. Delete
4. Pop - (Peek+Delete)
5. UpdatePriority ( element )
6. Search
*/

import (
	"fmt"

	// "github.com/pschlump/dbgo"
	// "github.com/pschlump/MiscLib"
	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap"
)

type priority_queue[T comparable.Comparable] struct {
	theHeap *heap.Heap[T]
}

// Create a new priority_queue and return it.
// Complexity is O(1).
func NewPriorityQueue[T comparable.Comparable]() (rv *priority_queue[T]) {
	// We don't have to "heapify" at this point becasue we start all heaps with an empty set of data.
	rv.theHeap = heap.NewHeap[T]()
	return
}

// Complexity O(1)
func (pq *priority_queue[T]) Peek() (rv *T) {
	return pq.theHeap.Peek()
}

// Complexity O(n log n)
func (pq *priority_queue[T]) Insert(n *T) {
	pq.theHeap.Push(n)
}

func (pq *priority_queue[T]) Pop() (rv *T) {
	return pq.theHeap.Pop()
}

// O(n log n)
func (pq *priority_queue[T]) Search(cmpVal *T) (rv *T, pos int, err error) {
	// Binary tree search to find matching node.
	return pq.theHeap.Search(cmpVal)
}

// Complexity O(n)
func (pq *priority_queue[T]) UpdatePriority(pos int, newVal *T) (found bool) {
	// check pos in range
	// update node at [pos]
	// re-heap-ify (down from pos)
	pq.theHeap.SetValue(pos, newVal)
	return
}

// Complexity O(n log n)
func (pq *priority_queue[T]) Delete(pos int) (err error) {
	// swap in node from leaf (last) to this potion
	// set last to nil
	// re-heap-ify (down from pos)
	x := pq.theHeap.Delete(pos)
	if x == nil {
		err = fmt.Errorf("Failed to delete, not found")
	}
	return
}

// Truncate removes all data from the heap.
// Complexity is O(1).
func (pq *priority_queue[T]) Truncate() {
	pq.theHeap = heap.NewHeap[T]()
}
