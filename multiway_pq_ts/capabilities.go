/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiway_pq_ts

import (
	"fmt"
	"slices"
	"strings"
)

// This file holds the convenience capabilities for the d-ary heap
// family — the §4.5 menu only: materialization, rendering, multiset
// equality and deep copy.  A heap is a multiset with a pop order, not
// a sequence and not a set, so there is deliberately no membership
// batch and no predicate bulk delete.  The observable order is the
// backing array — internal breadth-first tree order, the order All and
// the JSON marshal use — so Slice and String use that order.  Equal
// compares multisets via the sorted value sequence (ties safe),
// non-destructively, and is deliberately d-BLIND (a d=2 and a d=8 heap
// over the same multiset are equal — d shapes the tree, not the
// content).  Everything is additive and every new method is a total
// function on the read side: a nil heap and the zero value behave as
// an empty heap.
//
// Lock discipline (the heap_ts rules): Slice takes the snapshot All
// uses (the read lock is taken and released inside); everything after
// the snapshot — the StringFunc format callback, Equal's
// sort-and-compare, Clone's local rebuild — runs without any lock
// held, so a caller format function may safely call back into the
// heap.  Equal snapshots each side under its own lock and never holds
// two locks at once, so two heaps can be compared from concurrent
// goroutines in opposite directions without deadlock.  d and cmp are
// immutable after construction, so reading them without the lock is
// safe.  No new Nl methods: the capabilities are self-locking, not
// compounds.

// Slice returns a fresh slice holding the heap's elements in internal
// breadth-first tree order — the order All yields, the order of the
// backing array.  This is NOT sorted order; Pop is the sorted drain.
// The snapshot is taken under the read lock, so this is safe to call
// concurrently with any heap operation.  The slice is a copy: it
// aliases none of the heap's storage, and heap operations after the
// call never change it (nor does changing it change the heap).  A nil
// heap yields nil; an empty heap yields an empty (non-nil) slice.
// Complexity is O(n).
func (dh *DHeap[T]) Slice() []T {
	return dh.snapshot()
}

// String renders the heap as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in internal
// breadth-first tree order (All's order, not sorted order): "[a b c]".
// The elements are snapshotted under the read lock and the formatting
// runs without the lock.  A nil or empty heap renders as "[]".  For a
// custom per-element rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (dh *DHeap[T]) String() string {
	return dh.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in internal breadth-first tree
// order and the formatted strings joined inside square brackets.  The
// elements are snapshotted under the read lock and the format function
// runs without the lock, so it may safely call back into the heap.  A
// nil format function renders as "[]" (nothing is formatted, so a nil
// format is safe).
// Complexity is O(n) plus the cost of format.
func (dh *DHeap[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range dh.Slice() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same multiset of elements as
// the receiver: same length and the same sorted value sequence.  Two
// heaps built from the same multiset in different insertion orders —
// or at different branching factors d — hold different internal
// arrangements but are equal; ties are safe.  The comparison is
// non-destructive: each side is snapshotted, the copies are sorted —
// the receiver and other are never reordered.  Elements are compared
// through the heap's comparison function (never Go's ==), so a
// non-comparable element type is fine.  Two empty heaps are equal, a
// nil heap equals an empty heap, and Equal is symmetric on any pair
// that satisfies its precondition.
//
// Each heap is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines
// compare the same pair of heaps in opposite directions.
//
// Precondition: both heaps must use consistent comparison functions (a
// nil heap trivially does).  Functions are not comparable, so this
// cannot be checked — two heaps built by the same constructor over the
// same element type always satisfy it.
// Complexity is O(n log n).
func (dh *DHeap[T]) Equal(other *DHeap[T]) bool {
	if dh.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if dh.Len() == 0 {
		return true // both empty (either may be nil)
	}
	// A non-empty heap is necessarily constructor-built, so cmp is set.
	compare := dh.cmp
	a := dh.Slice()    // dh's lock, released before other is touched
	b := other.Slice() // other's lock, one snapshot
	slices.SortFunc(a, compare)
	slices.SortFunc(b, compare)
	for i := range a { // no lock held: the copies are compared
		if compare(a[i], b[i]) != 0 {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the heap: a fresh heap with the same
// branching factor and comparison function and the same elements in
// the same backing array — copying the array preserves the heap
// property exactly, so the clone's breadth-first order and its pop
// sequence are the receiver's.  No backing array is shared, so pushing
// to or popping from either heap afterwards never affects the other.
// The elements are snapshotted under the read lock and the rebuild
// runs on the local clone without any lock held.  A nil heap clones to
// nil; an empty heap clones to a fresh empty heap; a zero-value heap
// clones to a fresh empty zero-value-shaped heap (still without a
// comparison function, like the receiver).
// Complexity is O(n).
func (dh *DHeap[T]) Clone() *DHeap[T] {
	if dh == nil {
		return nil
	}
	items := dh.snapshot() // snapshot under the read lock
	dup := &DHeap[T]{d: dh.d, cmp: dh.cmp}
	dup.data = make([]T, len(items))
	copy(dup.data, items)
	return dup
}
