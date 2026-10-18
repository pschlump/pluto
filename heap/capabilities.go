/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package heap

import (
	"fmt"
	"slices"
	"strings"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 5) for the heap family — the §4.5
// menu only: materialization, rendering, multiset equality and deep copy.
// A heap is a multiset with a pop order, not a sequence and not a set, so
// there is deliberately no membership batch, no InsertSlice/DeleteSlice
// beyond what the package already has, and no predicate bulk delete.  The
// observable order of a heap is its backing array — internal
// breadth-first tree order, the order All yields and the JSON marshal
// uses (the note/implement-json.md precedent) — so Slice and String use
// that order; repeatedly calling Pop remains the way to consume a heap in
// sorted order.  Equal compares multisets, not layouts: two heaps over
// the same multiset may hold different internal arrangements (ties
// everywhere), so Equal sorts a copy of each side with the stored
// comparator and compares the sorted value sequences — non-destructively,
// the receiver is never reordered.  Everything is additive, and every new
// method is a total function on the read side: a nil heap and the zero
// value behave as an empty heap, exactly like the reads in heap.go.
// None of the new methods panics on a nil heap — and, unlike the sequence
// packages, none of them can panic on element comparison at all: equality
// goes through the stored comparison function, never through Go's ==, so
// a non-comparable element type is fine.

// Slice returns a fresh slice holding the heap's elements in internal
// breadth-first tree order — the order All yields, the order of the
// backing array.  This is NOT sorted order; Pop is the sorted drain.  The
// slice is a copy: it aliases none of the heap's storage, and heap
// operations after the call never change it (nor does changing it change
// the heap).  A nil heap yields nil; an empty heap yields an empty
// (non-nil) slice.
// Complexity is O(n).
func (hp *Heap[T]) Slice() []T {
	if hp == nil {
		return nil
	}
	items := make([]T, 0, len(hp.data))
	items = append(items, hp.data...)
	return items
}

// String renders the heap as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in internal
// breadth-first tree order (All's order, not sorted order): "[a b c]".
// A nil or empty heap renders as "[]".  For a custom per-element
// rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (hp *Heap[T]) String() string {
	return hp.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in internal breadth-first tree
// order and the formatted strings joined inside square brackets.  A nil
// format function renders as "[]" (nothing is formatted, so a nil format
// is safe).
// Complexity is O(n) plus the cost of format.
func (hp *Heap[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range hp.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same multiset of elements as the
// receiver: same length and the same sorted value sequence.  Two heaps
// built from the same multiset in different insertion orders hold
// different internal arrangements but are equal; ties are safe (the
// sorted sequences agree element for element exactly when the multisets
// do).  The comparison is non-destructive: each side is copied and the
// copy sorted — the receiver and other are never reordered.  Elements
// are compared through the heap's comparison function (never Go's ==),
// so a non-comparable element type is fine.  Two empty heaps are equal,
// a nil heap equals an empty heap, and Equal is symmetric on any pair
// that satisfies its precondition.
//
// Precondition: both heaps must use consistent comparison functions (a
// nil heap trivially does).  Functions are not comparable, so this cannot
// be checked — two heaps built by the same constructor over the same
// element type always satisfy it.
// Complexity is O(n log n).
func (hp *Heap[T]) Equal(other *Heap[T]) bool {
	if hp.Len() != other.Len() {
		return false
	}
	if hp.Len() == 0 {
		return true // both empty (either may be nil)
	}
	// A non-empty heap is necessarily constructor-built, so cmp is set.
	compare := hp.cmp
	a := hp.Slice()
	b := other.Slice()
	slices.SortFunc(a, compare)
	slices.SortFunc(b, compare)
	for i := range a {
		if compare(a[i], b[i]) != 0 {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the heap: a fresh heap with the same
// comparison function and the same elements in the same backing array —
// copying the array preserves the heap property exactly, so the clone's
// breadth-first order and its pop sequence are the receiver's.  No
// backing array is shared, so pushing to or popping from either heap
// afterwards never affects the other.  A nil heap clones to nil; an empty
// heap clones to a fresh empty heap.
// Complexity is O(n).
func (hp *Heap[T]) Clone() *Heap[T] {
	if hp == nil {
		return nil
	}
	dup := &Heap[T]{cmp: hp.cmp}
	dup.data = make([]T, len(hp.data))
	copy(dup.data, hp.data)
	return dup
}
