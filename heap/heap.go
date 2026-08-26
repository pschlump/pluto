// Package heap provides a generic min-heap.  Pop always removes and
// returns the minimum element.  The heap is stored as a slice in
// breadth-first tree order.
//
// It is the charon rework of github.com/pschlump/pluto/heap: the
// comparable.Comparable interface constraint is replaced with plain Go
// type parameters, elements are stored and returned by value, and the
// out-of-range index operations report not-found instead of panicking.
//
// The heap implementation is adapted from the standard library's
// container/heap.
//
// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package heap

import (
	"cmp"
	"fmt"
	"io"
)

// Complexity note.  The order uses 'n' where n = hp.Length().

// Heap is a generic min-heap.  Use NewHeap for naturally ordered element
// types (numbers, strings) or NewHeapFunc for a caller supplied
// comparison function.  The zero value is an empty heap.
type Heap[T any] struct {
	data []T

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewHeap and is
// handy for building custom comparison functions — including reversed
// ones, which turn the min-heap into a max-heap.
func Compare[T cmp.Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	default:
		return 0
	}
}

// NewHeap creates a new empty min-heap for any naturally ordered element
// type (all integers, floats and strings — cmp.Ordered).  Ordering uses
// the built-in < and > operators of T; no interface and no boxing is
// involved.
// Complexity is O(1).
func NewHeap[T cmp.Ordered]() *Heap[T] {
	return &Heap[T]{cmp: Compare[T]}
}

// NewHeapFunc creates a new empty min-heap that orders elements with the
// caller supplied comparison function fx.  fx must return a negative
// value if a sorts before b, 0 if the two are duplicates and a positive
// value if a sorts after b, and must order elements consistently.  A
// reversed comparison turns the heap into a max-heap.
// Complexity is O(1).
func NewHeapFunc[T any](fx func(a, b T) int) *Heap[T] {
	if fx == nil {
		panic("heap: NewHeapFunc called with a nil comparison function")
	}
	return &Heap[T]{cmp: fx}
}

// compare orders a and b, guarding against a zero-value heap that was
// not created by one of the constructors.
func (hp *Heap[T]) compare(a, b T) int {
	if hp.cmp == nil {
		panic("heap: no comparison function (create the heap with NewHeap or NewHeapFunc)")
	}
	return hp.cmp(a, b)
}

// Push appends the element x onto the end of the heap and re-orders the
// heap to be a heap.
// Push panics on a nil heap or on a zero-value heap (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty heap.
// Complexity is O(log n).
func (hp *Heap[T]) Push(x T) {
	if hp == nil {
		panic("heap: Push called on a nil heap")
	}
	if hp.cmp == nil {
		panic("heap: Push called on a heap with no comparison function (create the heap with NewHeap or NewHeapFunc)")
	}
	hp.data = append(hp.data, x)
	hp.up(len(hp.data) - 1) // Reorder to fix heap
}

// Pop removes and returns the minimum element.  Pop is the same as
// hp.Delete(0).  Pop on an empty heap reports false.
// Complexity is O(log n).
func (hp *Heap[T]) Pop() (rv T, found bool) {
	if hp == nil || len(hp.data) == 0 {
		return
	}
	n := len(hp.data) - 1
	hp.data[0], hp.data[n] = hp.data[n], hp.data[0] // Swap min to the end
	hp.down(0, n)                                   // Re-establish heap order
	rv = hp.data[n]
	var zero T
	hp.data[n] = zero // zero the slot so the GC can reclaim the popped element
	hp.data = hp.data[:n]
	if n == 0 {
		hp.data = nil // release the backing array on a full drain
	}
	return rv, true
}

// Peek returns the minimum element of the heap without removing it.
// Peek on an empty heap reports false.
// Complexity is O(1).
func (hp *Heap[T]) Peek() (rv T, found bool) {
	if hp == nil || len(hp.data) == 0 {
		return
	}
	return hp.data[0], true
}

// Truncate removes all elements from the heap, releasing the underlying
// storage so the GC can reclaim it.
// Complexity is O(1).
func (hp *Heap[T]) Truncate() {
	if hp == nil {
		return
	}
	hp.data = nil
}

// Delete removes and returns the element at the specified index `ii`
// from the heap.  It reports false if `ii` is out of range.
// Complexity is O(log n).
func (hp *Heap[T]) Delete(ii int) (rv T, found bool) {
	if hp == nil || ii < 0 || ii >= len(hp.data) {
		return
	}
	n := len(hp.data) - 1
	if n != ii {
		hp.data[ii], hp.data[n] = hp.data[n], hp.data[ii] // Swap ii with the last element
		if !hp.down(ii, n) {
			hp.up(ii)
		}
	}
	rv = hp.data[n]
	var zero T
	hp.data[n] = zero // zero the slot so the GC can reclaim the deleted element
	hp.data = hp.data[:n]
	if n == 0 {
		hp.data = nil // release the backing array on a full drain
	}
	return rv, true
}

// Fix re-establishes the heap ordering after the element at location
// `ii` has been replaced by `newValue`.  It reports false and does
// nothing if `ii` is out of range.  Replacing the element at `ii` and
// calling Fix is cheaper than a Delete(ii) followed by a Push(newValue).
// Complexity is O(log n).
func (hp *Heap[T]) Fix(ii int, newValue T) bool {
	if hp == nil || ii < 0 || ii >= len(hp.data) {
		return false
	}
	hp.data[ii] = newValue
	if !hp.down(ii, len(hp.data)) {
		hp.up(ii)
	}
	return true
}

// GetValue will return the value at index `ii` in the heap.  It reports
// false if `ii` is out of range.
// Complexity is O(1).
func (hp *Heap[T]) GetValue(ii int) (rv T, found bool) {
	if hp == nil || ii < 0 || ii >= len(hp.data) {
		return
	}
	return hp.data[ii], true
}

// SetValue replaces the element at index `ii` with `newValue` and
// re-establishes the heap ordering.  It reports false and does nothing
// if `ii` is out of range.  This is an alias for Fix.
// Complexity is O(log n).
func (hp *Heap[T]) SetValue(ii int, newValue T) bool {
	return hp.Fix(ii, newValue)
}

// Len will return the number of items in the heap.
// Complexity is O(1).
func (hp *Heap[T]) Len() int {
	if hp == nil {
		return 0
	}
	return len(hp.data)
}

// Length will return the number of items in the heap.  It is an alias for Len.
// Complexity is O(1).
func (hp *Heap[T]) Length() int {
	if hp == nil {
		return 0
	}
	return len(hp.data)
}

// Search performs a linear scan of the heap for an element equal to
// `cmpVal` (per the heap's comparison function).  It returns the element
// and its index, or (zero, -1, false) if no such element exists.  The
// probe only needs the fields that the comparison function reads.
// Complexity is O(n).
func (hp *Heap[T]) Search(cmpVal T) (rv T, pos int, found bool) {
	pos = -1
	if hp == nil {
		return
	}
	for ii := 0; ii < len(hp.data); ii++ {
		if hp.compare(hp.data[ii], cmpVal) == 0 {
			return hp.data[ii], ii, true
		}
	}
	return
}

// IsEmpty returns true if the heap is empty.
// Complexity is O(1).
func (hp *Heap[T]) IsEmpty() bool {
	return hp == nil || len(hp.data) == 0
}

// Lock is a no-op provided for API compatibility with a thread-safe heap
// twin.  This implementation is not safe for concurrent use.
func (hp *Heap[T]) Lock() {}

// Unlock is a no-op provided for API compatibility with a thread-safe heap
// twin.  This implementation is not safe for concurrent use.
func (hp *Heap[T]) Unlock() {}

func (hp *Heap[T]) up(j int) {
	for {
		i := (j - 1) / 2 // pick the parent
		if i == j || hp.compare(hp.data[j], hp.data[i]) > 0 {
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
		if j2 := j1 + 1; j2 < n && hp.compare(hp.data[j2], hp.data[j1]) < 0 {
			j = j2 // choose the right child
		}
		if hp.compare(hp.data[j], hp.data[i]) >= 0 {
			break
		}
		hp.data[i], hp.data[j] = hp.data[j], hp.data[i]
		i = j
	}
	return i > i0
}

// AppendHeap appends a new set of data to the heap and leaves the heap
// in a non-heap state.  After 1..n AppendHeap operations the heap must be
// rebuilt, e.g. with:
//
//	for i := h.Len()/2 - 1; i >= 0; i-- {
//		h.Heapify(h.Len(), i)
//	}
//
// AppendHeap follows the insert contract: the heap must have been created
// with NewHeap or NewHeapFunc.
// Complexity is O(m) where m = len(x).
func (hp *Heap[T]) AppendHeap(x []T) {
	if hp == nil {
		panic("heap: AppendHeap called on a nil heap")
	}
	if hp.cmp == nil {
		panic("heap: AppendHeap called on a heap with no comparison function (create the heap with NewHeap or NewHeapFunc)")
	}
	hp.data = append(hp.data, x...)
}

// Heapify re-establishes the min-heap ordering of the sub-tree rooted at index `i`,
// treating `n` as the effective size of the heap (only indexes < n are considered).
// It assumes the sub-trees below `i` already satisfy the heap property.
// To rebuild the entire heap, call it for i = Len()/2-1 down to 0 (see AppendHeap).
// Complexity is O(log n).
func (hp *Heap[T]) Heapify(n, i int) {
	if hp == nil {
		return
	}
	smallest := i // Initialize smallest as root
	l := 2*i + 1  // left = 2*i + 1
	r := 2*i + 2  // right = 2*i + 2
	if l < n && hp.compare(hp.data[l], hp.data[smallest]) < 0 {
		smallest = l
	}
	if r < n && hp.compare(hp.data[r], hp.data[smallest]) < 0 {
		smallest = r
	}
	if smallest != i {
		hp.data[i], hp.data[smallest] = hp.data[smallest], hp.data[i]
		hp.Heapify(n, smallest) // Recursively heapify the affected sub-tree
	}
}

// Dump writes the contents of the heap (in internal slice order — not
// sorted order) to `fp`, one indexed line per element.  An empty heap
// produces no output.  It is a debugging aid; repeatedly calling Pop is
// the way to consume a heap in sorted order.
// Complexity is O(n).
func (hp *Heap[T]) Dump(fp io.Writer) {
	if hp == nil || len(hp.data) == 0 {
		return
	}
	_, _ = fmt.Fprintf(fp, "Heap length=%d\n", len(hp.data))
	for i, v := range hp.data {
		_, _ = fmt.Fprintf(fp, "%d: %+v\n", i, v)
	}
}
