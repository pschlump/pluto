// Package heap_ts provides a thread-safe generic min-heap for any type that
// implements comparable.Comparable.  Pop always removes and returns the
// minimum element.  The heap is stored as a slice in breadth-first tree
// order.
//
// Every operation is guarded by an internal sync.RWMutex.  It has the exact
// same interface as the (non-thread-safe) heap package.
//
// The implementation is adapted from the standard library's container/heap.
//
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright (C) 2021 Philip Schlump. All rights reserved.
package heap_ts

import (
	"fmt"
	"io"
	"sync"

	"github.com/pschlump/dbgo"
	"github.com/pschlump/pluto/comparable"
)

//
// Complexity note.  The order uses 'n' where n = hp.Length().
//

// The heap data is stored in a slice of type *T
type Heap[T comparable.Comparable] struct {
	data []*T
	mu   sync.RWMutex
}

// Create a new heap and return it.
// Complexity is O(1).
func NewHeap[T comparable.Comparable]() *Heap[T] {
	// We don't have to "heapify" at this point becasue we start all heaps with an empty set of data.
	return &Heap[T]{}
}

// Push appends the element x onto the end of the heap and re-orders the heap to be a heap.
// Complexity is O(log n).
func (hp *Heap[T]) Push(x *T) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.data = append(hp.data, x) // hp.Push()
	hp.up(len(hp.data) - 1)      // Reorder to fix heap
}

// Pop removes and returns the minimum element (using comparable.Compare).
// Pop is the same as hp.Delete(0).  Pop on an empty heap returns nil.
// Complexity is O(log n).
func (hp *Heap[T]) Pop() (rv *T) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	n := len(hp.data) - 1
	if n < 0 {
		return nil
	}
	hp.data[0], hp.data[n] = hp.data[n], hp.data[0] // Swap min to the end
	hp.down(0, n)                                   // Re-establish heap order
	rv = hp.data[n]
	hp.data[n] = nil // zero the slot so the GC can reclaim the popped element
	hp.data = hp.data[:n]
	return
}

// Peek returns the minimum element of the heap without removing it.
// Peek on an empty heap returns nil.
//
// Note: the returned pointer aliases data stored in the heap; treat it as
// read-only.  The element may be removed by another goroutine at any time.
// Complexity is O(1).
func (hp *Heap[T]) Peek() (rv *T) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	if len(hp.data) > 0 {
		return hp.data[0]
	}
	return nil
}

// Truncate removes all elements from the heap, releasing the underlying
// storage so the GC can reclaim it.
// Complexity is O(1).
func (hp *Heap[T]) Truncate() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.data = nil
}

// Delete removes and returns the element at the specified index `ii` from the heap.
// It panics if `ii` is out of range.
// Complexity is O(log n).
func (hp *Heap[T]) Delete(ii int) (rv *T) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if ii < 0 || ii >= len(hp.data) {
		panic("heap index out of range")
	}
	n := len(hp.data) - 1
	if n != ii {
		hp.data[ii], hp.data[n] = hp.data[n], hp.data[ii] // Swap ii with the last element
		if !hp.down(ii, n) {
			hp.up(ii)
		}
	}
	rv = hp.data[n]
	hp.data[n] = nil // zero the slot so the GC can reclaim the deleted element
	hp.data = hp.data[:n]
	return
}

// Fix re-establishes the heap ordering after the element at location `ii` has
// been replaced by `newValue`.  It panics if `ii` is out of range.
// Replacing the element at `ii` and calling Fix is cheaper than a Delete(ii)
// followed by a Push(newValue).
// Complexity is O(log n).
func (hp *Heap[T]) Fix(ii int, newValue *T) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if ii < 0 || ii >= len(hp.data) {
		panic("heap index out of range")
	}
	hp.data[ii] = newValue
	if !hp.down(ii, len(hp.data)) {
		hp.up(ii)
	}
}

// GetValue will return the value at index `ii` in the heap.
//
// Note: the returned pointer aliases data stored in the heap; treat it as
// read-only.  The index is only meaningful while no other goroutine mutates
// the heap.
// Complexity is O(1).
func (hp *Heap[T]) GetValue(ii int) (value *T) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	if ii < 0 || ii >= len(hp.data) {
		panic("heap index out of range")
	}
	return hp.data[ii]
}

// SetValue replaces the element at index `ii` with `newValue` and
// re-establishes the heap ordering.  It panics if `ii` is out of range.
// Complexity is O(log n).
func (hp *Heap[T]) SetValue(ii int, newValue *T) {
	hp.Fix(ii, newValue)
}

// Len will return the number of items in the heap.
// Complexity is O(1).
func (hp *Heap[T]) Len() int {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	return len(hp.data)
}

// Length will return the number of items in the heap.  It is an alias for Len.
// Complexity is O(1).
func (hp *Heap[T]) Length() int {
	return hp.Len()
}

// Search performs a linear scan of the heap for an element equal to `cmpVal`
// (per comparable.Compare).  It returns the element and its index, or
// nil, -1, nil if no such element exists.  `err` is always nil and retained
// for interface compatibility.
//
// Note: the returned pointer aliases data stored in the heap; treat it as
// read-only.  The returned index may be invalidated by any concurrent
// mutation of the heap.
// Complexity is O(n).
func (hp *Heap[T]) Search(cmpVal *T) (rv *T, pos int, err error) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	pos = -1
	for ii := 0; ii < len(hp.data); ii++ {
		c := (*(hp.data[ii])).Compare(*cmpVal)
		if c == 0 {
			rv, pos = hp.data[ii], ii
			return
		}
	}
	return
}

// Lock takes the heap's write lock, allowing a group of operations to be
// performed atomically.  While the lock is held only call the Nl-prefixed
// (no-lock) methods; calling a regular method will deadlock.  Pair every
// Lock with a corresponding Unlock.
func (hp *Heap[T]) Lock() {
	hp.mu.Lock()
}

// Unlock releases the write lock taken by Lock.
func (hp *Heap[T]) Unlock() {
	hp.mu.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (hp *Heap[T]) NlLen() int {
	return len(hp.data)
}

// NlGetValue is GetValue without locking; call it only while holding Lock.
func (hp *Heap[T]) NlGetValue(ii int) (value *T) {
	if ii < 0 || ii >= len(hp.data) {
		panic("heap index out of range")
	}
	return hp.data[ii]
}

// NlPush is Push without locking; call it only while holding Lock.
func (hp *Heap[T]) NlPush(x *T) {
	hp.data = append(hp.data, x)
	hp.up(len(hp.data) - 1)
}

// NlPop is Pop without locking; call it only while holding Lock.
func (hp *Heap[T]) NlPop() (rv *T) {
	n := len(hp.data) - 1
	if n < 0 {
		return nil
	}
	hp.data[0], hp.data[n] = hp.data[n], hp.data[0]
	hp.down(0, n)
	rv = hp.data[n]
	hp.data[n] = nil
	hp.data = hp.data[:n]
	return
}

// NlDelete is Delete without locking; call it only while holding Lock.
func (hp *Heap[T]) NlDelete(ii int) (rv *T) {
	if ii < 0 || ii >= len(hp.data) {
		panic("heap index out of range")
	}
	n := len(hp.data) - 1
	if n != ii {
		hp.data[ii], hp.data[n] = hp.data[n], hp.data[ii]
		if !hp.down(ii, n) {
			hp.up(ii)
		}
	}
	rv = hp.data[n]
	hp.data[n] = nil
	hp.data = hp.data[:n]
	return
}

// NlFix is Fix without locking; call it only while holding Lock.
func (hp *Heap[T]) NlFix(ii int, newValue *T) {
	if ii < 0 || ii >= len(hp.data) {
		panic("heap index out of range")
	}
	hp.data[ii] = newValue
	if !hp.down(ii, len(hp.data)) {
		hp.up(ii)
	}
}

// snapshot returns a copy of the heap's backing slice, taken under the read
// lock.
func (hp *Heap[T]) snapshot() []*T {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	rv := make([]*T, len(hp.data))
	copy(rv, hp.data)
	return rv
}

func (hp *Heap[T]) up(j int) {
	for {
		i := (j - 1) / 2 // pick the parent
		c := (*(hp.data[j])).Compare(*(hp.data[i]))
		if i == j || c > 0 {
			break
		}
		hp.data[i], hp.data[j] = hp.data[j], hp.data[i]
		j = i
	}
}

func (hp *Heap[T]) down(i0, n int) (rv bool) {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 guards against int overflow
			break
		}
		j := j1 // choose the left child
		if j2 := j1 + 1; j2 < n && (*(hp.data[j2])).Compare(*(hp.data[j1])) < 0 {
			j = j2 // choose the right child
		}
		if c := (*(hp.data[j])).Compare(*(hp.data[i])); c >= 0 {
			break
		}
		hp.data[i], hp.data[j] = hp.data[j], hp.data[i]
		i = j
	}
	rv = i > i0
	return
}

// AppendHeap appends a new set of data to the heap (and leaves the heap in a non-heap state).
// After 1..n AppendHeap operations the heap must be rebuilt, e.g. with:
//
//	for i := h.Len()/2 - 1; i >= 0; i-- {
//		h.Heapify(h.Len(), i)
//	}
//
// Complexity is O(m) where m = len(x).
func (hp *Heap[T]) AppendHeap(x []*T) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.data = append(hp.data, x...)
}

// Heapify re-establishes the min-heap ordering of the sub-tree rooted at index `i`,
// treating `n` as the effective size of the heap (only indexes < n are considered).
// It assumes the sub-trees below `i` already satisfy the heap property.
// To rebuild the entire heap, call it for i = Len()/2-1 down to 0 (see AppendHeap).
// Complexity is O(log n).
func (hp *Heap[T]) Heapify(n, i int) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	hp.heapify(n, i)
}

// heapify is Heapify without locking; the write lock must already be held.
func (hp *Heap[T]) heapify(n, i int) {
	smallest := i // Initialize smallest as root
	l := 2*i + 1  // left = 2*i + 1
	r := 2*i + 2  // right = 2*i + 2
	if l < n && (*(hp.data[l])).Compare(*(hp.data[smallest])) < 0 {
		smallest = l
	}
	if r < n && (*(hp.data[r])).Compare(*(hp.data[smallest])) < 0 {
		smallest = r
	}
	if smallest != i {
		hp.data[i], hp.data[smallest] = hp.data[smallest], hp.data[i]
		hp.heapify(n, smallest) // Recursively heapify the affected sub-tree
	}
}

// Dump writes the contents of the heap (in internal slice order) to `fp`.
func (hp *Heap[T]) Dump(fp io.Writer) {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	_, _ = fmt.Fprintf(fp, "%s\n", dbgo.SVarI(hp.data))
}
