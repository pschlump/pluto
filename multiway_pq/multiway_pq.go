// Package multiway_pq provides a generic d-ary min-heap — the multiway
// heap of Sedgwick's Algorithms §9 "Beyond" (MultiwayMinPQ /
// IndexMultiwayMinPQ), the practical variant for large heaps.  Pop
// always removes and returns the minimum element; the heap is stored
// as a slice in breadth-first tree order exactly like pluto's heap,
// but each node has d children instead of 2.
//
// A larger d makes the tree shallower: sift-UP operations (Push, a
// decrease toward the root) compare against one node per level —
// O(log_d n) comparisons instead of the binary heap's O(log₂ n) — at
// the price of sift-DOWN operations scanning all d children per level
// to find the smallest (O(d·log_d n)).  The array also gets denser and
// more cache-friendly as d grows.  d = 2 is the plain binary heap (a
// direct performance baseline); the d-ary heap's advantage is the
// cheaper decrease-key, which is why algs4 positions it behind
// dense-graph Dijkstra and large schedulers — see the benchmark table
// in the README for where each d actually lands.
//
// The element type implements no interface: ordering is supplied as a
// plain comparison function over type parameters, elements are stored
// and returned by value, and the out-of-range index operations report
// not-found instead of panicking.
//
// Set-convenience capabilities (capabilities.go): Slice, String,
// StringFunc, Equal and Clone.  Slice and String materialize the
// elements in internal breadth-first backing order — the order All
// yields, NOT sorted order; Pop is the sorted drain.  Equal is MULTISET
// equality — same length and the same sorted value sequence, so heaps
// built from the same elements in any insertion order (at any branching
// factor d) are equal — and is non-destructive; it compares through the
// stored comparison function, never Go's ==, so a non-comparable
// element type is fine.  Clone copies the backing array, preserving the
// exact layout and pop sequence.  The capability methods are total
// functions — none of them panics on a nil or zero-value heap.
//
// A DHeap is not safe for concurrent use.
//
// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
package multiway_pq

import (
	"cmp"
	"fmt"
	"io"
)

// Complexity note.  The orders use 'n' where n = dh.Length() and 'd'
// for the branching factor fixed at construction.  Sift-up is
// O(log_d n) comparisons; sift-down is O(d·log_d n).

// DHeap is a generic d-ary min-heap.  Use NewDHeap for naturally
// ordered element types (numbers, strings) or NewDHeapFunc for a
// caller supplied comparison function.  The zero value is not usable
// for writes (no branching factor, no comparison function); every read
// treats it as an empty heap.
type DHeap[T any] struct {
	data []T

	// d is the branching factor: node i's children live at d*i+1 .. d*i+d
	// and its parent at (i-1)/d.  Fixed at construction, d >= 2.
	d int

	// cmp orders two elements: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewDHeap and is
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

// NewDHeap creates a new empty d-ary min-heap with branching factor d
// (d >= 2) for any naturally ordered element type (all integers,
// floats and strings — cmp.Ordered).  Ordering uses the built-in < and
// > operators of T; no interface and no boxing is involved.
// Complexity is O(1).
func NewDHeap[T cmp.Ordered](d int) *DHeap[T] {
	return NewDHeapFunc(d, Compare[T])
}

// NewDHeapFunc creates a new empty d-ary min-heap with branching
// factor d (d >= 2) that orders elements with the caller supplied
// comparison function fx.  fx must return a negative value if a sorts
// before b, 0 if the two are duplicates and a positive value if a
// sorts after b, and must order elements consistently.  A reversed
// comparison turns the heap into a max-heap.
// Complexity is O(1).
func NewDHeapFunc[T any](d int, fx func(a, b T) int) *DHeap[T] {
	if fx == nil {
		panic("multiway_pq: NewDHeapFunc called with a nil comparison function")
	}
	if d < 2 {
		panic("multiway_pq: NewDHeap/NewDHeapFunc called with d < 2 (a heap needs at least two children per node)")
	}
	return &DHeap[T]{d: d, cmp: fx}
}

// compare orders a and b, guarding against a zero-value heap that was
// not created by one of the constructors.
func (dh *DHeap[T]) compare(a, b T) int {
	if dh.cmp == nil {
		panic("multiway_pq: no comparison function (create the heap with NewDHeap or NewDHeapFunc)")
	}
	return dh.cmp(a, b)
}

// Push appends the element x onto the end of the heap and re-orders
// the heap to be a heap.
// Push panics on a nil heap or on a zero-value heap (no comparison
// function); these are the only panics on non-constructor calls in the
// package — every other operation treats both as an empty heap.
// Complexity is O(log_d n) comparisons — one per level, cheaper than
// the binary heap's log₂ n.
func (dh *DHeap[T]) Push(x T) {
	if dh == nil {
		panic("multiway_pq: Push called on a nil heap")
	}
	if dh.cmp == nil {
		panic("multiway_pq: Push called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)")
	}
	dh.data = append(dh.data, x)
	dh.up(len(dh.data) - 1) // Reorder to fix heap
}

// Pop removes and returns the minimum element.  Pop is the same as
// dh.Delete(0).  Pop on an empty heap reports false.
// Complexity is O(d·log_d n).
func (dh *DHeap[T]) Pop() (rv T, found bool) {
	if dh == nil || len(dh.data) == 0 {
		return
	}
	n := len(dh.data) - 1
	dh.data[0], dh.data[n] = dh.data[n], dh.data[0] // Swap min to the end
	dh.down(0, n)                                   // Re-establish heap order
	rv = dh.data[n]
	var zero T
	dh.data[n] = zero // zero the slot so the GC can reclaim the popped element
	dh.data = dh.data[:n]
	if n == 0 {
		dh.data = nil // release the backing array on a full drain
	}
	return rv, true
}

// Peek returns the minimum element of the heap without removing it.
// Peek on an empty heap reports false.
// Complexity is O(1).
func (dh *DHeap[T]) Peek() (rv T, found bool) {
	if dh == nil || len(dh.data) == 0 {
		return
	}
	return dh.data[0], true
}

// Truncate removes all elements from the heap, releasing the underlying
// storage so the GC can reclaim it.
// Complexity is O(1).
func (dh *DHeap[T]) Truncate() {
	if dh == nil {
		return
	}
	dh.data = nil
}

// Delete removes and returns the element at the specified index `ii`
// from the heap.  It reports false if `ii` is out of range.
// Complexity is O(d·log_d n).
func (dh *DHeap[T]) Delete(ii int) (rv T, found bool) {
	if dh == nil || ii < 0 || ii >= len(dh.data) {
		return
	}
	n := len(dh.data) - 1
	if n != ii {
		dh.data[ii], dh.data[n] = dh.data[n], dh.data[ii] // Swap ii with the last element
		if !dh.down(ii, n) {
			dh.up(ii)
		}
	}
	rv = dh.data[n]
	var zero T
	dh.data[n] = zero // zero the slot so the GC can reclaim the deleted element
	dh.data = dh.data[:n]
	if n == 0 {
		dh.data = nil // release the backing array on a full drain
	}
	return rv, true
}

// Fix re-establishes the heap ordering after the element at location
// `ii` has been replaced by `newValue`.  It reports false and does
// nothing if `ii` is out of range.  Replacing the element at `ii` and
// calling Fix is cheaper than a Delete(ii) followed by a Push(newValue).
// Complexity is O(d·log_d n) worst case; a decrease toward the root
// costs only O(log_d n).
func (dh *DHeap[T]) Fix(ii int, newValue T) bool {
	if dh == nil || ii < 0 || ii >= len(dh.data) {
		return false
	}
	dh.data[ii] = newValue
	if !dh.down(ii, len(dh.data)) {
		dh.up(ii)
	}
	return true
}

// GetValue will return the value at index `ii` in the heap.  It reports
// false if `ii` is out of range.
// Complexity is O(1).
func (dh *DHeap[T]) GetValue(ii int) (rv T, found bool) {
	if dh == nil || ii < 0 || ii >= len(dh.data) {
		return
	}
	return dh.data[ii], true
}

// SetValue replaces the element at index `ii` with `newValue` and
// re-establishes the heap ordering.  It reports false and does nothing
// if `ii` is out of range.  This is an alias for Fix.
// Complexity is O(d·log_d n).
func (dh *DHeap[T]) SetValue(ii int, newValue T) bool {
	return dh.Fix(ii, newValue)
}

// Len will return the number of items in the heap.
// Complexity is O(1).
func (dh *DHeap[T]) Len() int {
	if dh == nil {
		return 0
	}
	return len(dh.data)
}

// Length will return the number of items in the heap.  It is an alias for Len.
// Complexity is O(1).
func (dh *DHeap[T]) Length() int {
	if dh == nil {
		return 0
	}
	return len(dh.data)
}

// Search performs a linear scan of the heap for an element equal to
// `cmpVal` (per the heap's comparison function).  It returns the element
// and its index, or (zero, -1, false) if no such element exists.  The
// probe only needs the fields that the comparison function reads.
// Complexity is O(n).
func (dh *DHeap[T]) Search(cmpVal T) (rv T, pos int, found bool) {
	pos = -1
	if dh == nil {
		return
	}
	for ii := 0; ii < len(dh.data); ii++ {
		if dh.compare(dh.data[ii], cmpVal) == 0 {
			return dh.data[ii], ii, true
		}
	}
	return
}

// IsEmpty returns true if the heap is empty.
// Complexity is O(1).
func (dh *DHeap[T]) IsEmpty() bool {
	return dh == nil || len(dh.data) == 0
}

// Lock is a no-op kept for source compatibility with the heap_ts
// surface shape.  This implementation is not safe for concurrent use —
// the multiway_pq_ts twin has the real lock.
func (dh *DHeap[T]) Lock() {}

// Unlock is a no-op kept for source compatibility with the heap_ts
// surface shape.  This implementation is not safe for concurrent use —
// the multiway_pq_ts twin has the real lock.
func (dh *DHeap[T]) Unlock() {}

// up sifts j toward the root while it orders before its parent — one
// comparison per level, d of which fit in each binary level.
func (dh *DHeap[T]) up(j int) {
	for {
		i := (j - 1) / dh.d // pick the parent
		if i == j || dh.compare(dh.data[j], dh.data[i]) > 0 {
			break
		}
		dh.data[i], dh.data[j] = dh.data[j], dh.data[i]
		j = i
	}
}

// down sifts i0 away from the root while a child orders before it,
// swapping with the SMALLEST of the d children — d-1 comparisons per
// level, the price of the shallower tree.
func (dh *DHeap[T]) down(i0, n int) (rv bool) {
	i := i0
	for {
		j1 := dh.d*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 guards against int overflow
			break
		}
		j := j1 // choose the first child
		for k := j1 + 1; k < j1+dh.d && k < n; k++ {
			if dh.compare(dh.data[k], dh.data[j]) < 0 {
				j = k // choose the smallest child
			}
		}
		if dh.compare(dh.data[j], dh.data[i]) >= 0 {
			break
		}
		dh.data[i], dh.data[j] = dh.data[j], dh.data[i]
		i = j
	}
	return i > i0
}

// AppendHeap appends a new set of data to the heap and leaves the heap
// in a non-heap state.  After 1..n AppendHeap operations the heap must be
// rebuilt, e.g. with:
//
//	for i := (h.Len()-2)/h.D(); i >= 0; i-- {
//		h.Heapify(h.Len(), i)
//	}
//
// AppendHeap follows the insert contract: the heap must have been created
// with NewDHeap or NewDHeapFunc.
// Complexity is O(m) where m = len(x).
func (dh *DHeap[T]) AppendHeap(x []T) {
	if dh == nil {
		panic("multiway_pq: AppendHeap called on a nil heap")
	}
	if dh.cmp == nil {
		panic("multiway_pq: AppendHeap called on a heap with no comparison function (create the heap with NewDHeap or NewDHeapFunc)")
	}
	dh.data = append(dh.data, x...)
}

// D returns the branching factor fixed at construction (d >= 2).
// Complexity is O(1).
func (dh *DHeap[T]) D() int {
	if dh == nil || dh.d < 2 {
		return 2 // a nil/zero-value heap behaves as the empty d=2 case
	}
	return dh.d
}

// Heapify re-establishes the min-heap ordering of the sub-tree rooted at index `i`,
// treating `n` as the effective size of the heap (only indexes < n are considered).
// It assumes the sub-trees below `i` already satisfy the heap property.
// To rebuild the entire heap, call it for i = (Len()-2)/D() down to 0 (see AppendHeap).
// Complexity is O(d·log_d n).
func (dh *DHeap[T]) Heapify(n, i int) {
	if dh == nil {
		return
	}
	smallest := i // Initialize smallest as root
	for k := dh.d*i + 1; k <= dh.d*i+dh.d && k < n; k++ {
		if dh.compare(dh.data[k], dh.data[smallest]) < 0 {
			smallest = k
		}
	}
	if smallest != i {
		dh.data[i], dh.data[smallest] = dh.data[smallest], dh.data[i]
		dh.Heapify(n, smallest) // Recursively heapify the affected sub-tree
	}
}

// Dump writes the contents of the heap (in internal slice order — not
// sorted order) to `fp`, one indexed line per element.  An empty heap
// produces no output.  It is a debugging aid; repeatedly calling Pop is
// the way to consume a heap in sorted order.
// Complexity is O(n).
func (dh *DHeap[T]) Dump(fp io.Writer) {
	if dh == nil || len(dh.data) == 0 {
		return
	}
	_, _ = fmt.Fprintf(fp, "DHeap(d=%d) length=%d\n", dh.D(), len(dh.data))
	for i, v := range dh.data {
		_, _ = fmt.Fprintf(fp, "%d: %+v\n", i, v)
	}
}
