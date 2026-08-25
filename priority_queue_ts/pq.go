// Package priority_queue_ts implements a thread-safe generic priority queue
// backed by the thread-safe binary min-heap in heap_ts.
//
// The queue supports the following operations:
//
//  1. Peek           - look at the lowest-priority (minimum) element
//  2. Insert         - add an element
//  3. Delete         - remove the element at a given position
//  4. Pop            - Peek + Delete of the minimum element
//  5. UpdatePriority - replace the element at a given position and re-heapify
//  6. Search         - find an element by value
//
// Elements are stored as *T and must implement comparable.Comparable.
// It has the exact same interface as the (non-thread-safe) priority_queue
// package; every operation is guarded by the sync.RWMutex inside heap_ts.
//
// Note: pointers returned by Peek/Search/Pop alias data stored in the queue;
// treat them as read-only.  Positions returned by Search may be invalidated
// by any concurrent mutation of the queue.
package priority_queue_ts

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/comparable"
	"github.com/pschlump/pluto/heap_ts"
)

// PriorityQueue is a thread-safe priority queue of *T values ordered by
// comparable.Comparable.Compare.  Create one with NewPriorityQueue.
type PriorityQueue[T comparable.Comparable] struct {
	h *heap_ts.Heap[T]
}

// NewPriorityQueue creates a new, empty priority queue and returns it.
// Complexity is O(1).
func NewPriorityQueue[T comparable.Comparable]() (rv *PriorityQueue[T]) {
	// We don't have to "heapify" at this point because we start all heaps with an empty set of data.
	rv = &PriorityQueue[T]{}
	rv.h = heap_ts.NewHeap[T]()
	return
}

// Len returns the number of elements in the queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Len() int {
	return pq.h.Len()
}

// IsEmpty returns true if the queue has no elements.
// Complexity is O(1).
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.h.Len() == 0
}

// Peek returns the minimum element in the queue without removing it,
// or nil if the queue is empty.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Peek() (rv *T) {
	return pq.h.Peek()
}

// Insert adds the element n to the queue.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Insert(n *T) {
	pq.h.Push(n)
}

// Pop removes and returns the minimum element in the queue,
// or nil if the queue is empty.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Pop() (rv *T) {
	return pq.h.Pop()
}

// Search finds the first element that compares equal (Compare == 0) to
// cmpVal and returns it along with its position in the underlying heap.
// If no element matches, rv is nil and err is non-nil.
//
// Note: the returned position may be invalidated by any concurrent mutation
// of the queue; use Lock/Unlock with the Nl-prefixed methods for atomic
// search-then-update sequences.
// Complexity is O(n).
func (pq *PriorityQueue[T]) Search(cmpVal *T) (rv *T, pos int, err error) {
	rv, pos, _ = pq.h.Search(cmpVal)
	if rv == nil {
		err = fmt.Errorf("not found")
	}
	return
}

// UpdatePriority replaces the element at position pos (a position previously
// returned by Search) with newVal and re-establishes the heap ordering.
// It returns true if pos was valid, false otherwise.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) UpdatePriority(pos int, newVal *T) (found bool) {
	if pos < 0 || pos >= pq.h.Len() {
		return false
	}
	pq.h.Fix(pos, newVal)
	return true
}

// Delete removes the element at position pos (a position previously returned
// by Search) from the queue. It returns a non-nil error if pos is out of
// range.
// Complexity is O(log n).
func (pq *PriorityQueue[T]) Delete(pos int) (err error) {
	n := pq.h.Len()
	if pos < 0 || pos >= n {
		return fmt.Errorf("failed to delete, position %d out of range [0..%d)", pos, n)
	}
	pq.h.Delete(pos)
	return nil
}

// Truncate removes all data from the queue.
// Complexity is O(1).
func (pq *PriorityQueue[T]) Truncate() {
	pq.h.Truncate()
}

// Lock takes the queue's write lock, allowing a group of operations to be
// performed atomically (e.g. a Search-then-UpdatePriority sequence).  While
// the lock is held only call the Nl-prefixed (no-lock) methods; calling a
// regular method will deadlock.  Pair every Lock with a corresponding
// Unlock.
func (pq *PriorityQueue[T]) Lock() {
	pq.h.Lock()
}

// Unlock releases the write lock taken by Lock.
func (pq *PriorityQueue[T]) Unlock() {
	pq.h.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlLen() int {
	return pq.h.NlLen()
}

// NlIsEmpty is IsEmpty without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlIsEmpty() bool {
	return pq.h.NlLen() == 0
}

// NlPeek is Peek without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlPeek() (rv *T) {
	if pq.h.NlLen() == 0 {
		return nil
	}
	return pq.h.NlGetValue(0)
}

// NlInsert is Insert without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlInsert(n *T) {
	pq.h.NlPush(n)
}

// NlPop is Pop without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlPop() (rv *T) {
	return pq.h.NlPop()
}

// NlSearch is Search without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlSearch(cmpVal *T) (rv *T, pos int, err error) {
	pos = -1
	for ii := 0; ii < pq.h.NlLen(); ii++ {
		if (*pq.h.NlGetValue(ii)).Compare(*cmpVal) == 0 {
			return pq.h.NlGetValue(ii), ii, nil
		}
	}
	return nil, -1, fmt.Errorf("not found")
}

// NlUpdatePriority is UpdatePriority without locking; call it only while
// holding Lock.
func (pq *PriorityQueue[T]) NlUpdatePriority(pos int, newVal *T) (found bool) {
	if pos < 0 || pos >= pq.h.NlLen() {
		return false
	}
	pq.h.NlFix(pos, newVal)
	return true
}

// NlDelete is Delete without locking; call it only while holding Lock.
func (pq *PriorityQueue[T]) NlDelete(pos int) (err error) {
	n := pq.h.NlLen()
	if pos < 0 || pos >= n {
		return fmt.Errorf("failed to delete, position %d out of range [0..%d)", pos, n)
	}
	pq.h.NlDelete(pos)
	return nil
}

// All returns an iterator (Go 1.23+ range-over-func) that yields the
// elements of the queue in priority order, minimum element first.
//
// The iteration is non-destructive: it operates on a private copy of the
// heap built from a snapshot taken under the write lock, so the queue is
// unchanged afterwards and it is safe to call queue methods from inside the
// loop body.  Elements inserted or removed after the snapshot do not affect
// the sequence.
// Complexity is O(n log n) to build the snapshot, then O(1) per element
// amortized.
//
//	for item := range pq.All() {
//		fmt.Println(item)
//	}
func (pq *PriorityQueue[T]) All() iter.Seq[*T] {
	return func(yield func(*T) bool) {
		snapshot := heap_ts.NewHeap[T]()
		pq.h.Lock()
		for i := 0; i < pq.h.NlLen(); i++ {
			snapshot.NlPush(pq.h.NlGetValue(i))
		}
		pq.h.Unlock()
		for {
			v := snapshot.Pop()
			if v == nil {
				return
			}
			if !yield(v) {
				return
			}
		}
	}
}
