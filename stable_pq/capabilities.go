/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stable_pq

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pschlump/pluto/heap"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 5) for the heap family — the §4.5
// menu only: materialization, rendering, multiset equality and deep copy.
// A priority queue is a multiset with a pop order, not a sequence and not
// a set, so there is deliberately no membership batch and no predicate
// bulk delete.  The observable materialization order is All's order —
// (key, insertion) order: minimum key first and, among equal keys, oldest
// first, the exact order Pop would produce — so Slice and String use
// that order.  Equal compares the KEY multiset and deliberately IGNORES
// the insertion stamps: the stamps are internal bookkeeping that makes
// the pop order total, not content — two queues over the same key
// multiset inserted in different orders carry different stamps but are
// equal.  Clone copies the stamps too, so the clone's pop order — FIFO
// among ties included — is the receiver's.  Everything is additive, and
// every new method is a total function on the read side: a nil queue and
// the zero value behave as an empty queue, exactly like the reads in
// stable_pq.go.  None of the new methods panics on a nil queue — and
// none of them can panic on element comparison at all: equality goes
// through the stored comparison function, never through Go's ==, so a
// non-comparable key type is fine.

// Slice returns a fresh slice holding the queue's keys in (key,
// insertion) order — minimum key first and, among equal keys, oldest
// first — the order All yields, the exact order Pop would produce.
// The slice is a copy: it aliases none of the queue's storage, and
// queue operations after the call never change it.  A nil queue yields
// nil; an empty queue yields an empty (non-nil) slice.
// Complexity is O(n log n) — the snapshot drain All performs.
func (pq *StablePQ[T]) Slice() []T {
	if pq == nil {
		return nil
	}
	items := make([]T, 0, pq.Len())
	for v := range pq.All() {
		items = append(items, v)
	}
	return items
}

// String renders the queue as its keys formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in (key, insertion)
// order (All's order): "[a b c]".  A nil or empty queue renders as "[]".
// For a custom per-element rendering use StringFunc.
// Complexity is O(n log n) plus the cost of formatting each key.
func (pq *StablePQ[T]) String() string {
	return pq.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the keys are rendered in (key, insertion) order and the
// formatted strings joined inside square brackets.  A nil format
// function renders as "[]" (nothing is formatted, so a nil format is
// safe).
// Complexity is O(n log n) plus the cost of format.
func (pq *StablePQ[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range pq.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same multiset of keys as the
// receiver: same length and the same sorted key sequence.  The
// insertion stamps are deliberately ignored — they are the internal
// tie-breaking bookkeeping, not content, so two queues over the same
// key multiset inserted in different orders (hence with different
// stamps) are equal.  Ties are safe (the sorted key sequences agree
// element for element exactly when the multisets do).  The comparison is
// non-destructive: each side is copied and the copy sorted — the
// receiver and other are never reordered.  Keys are compared through
// the queue's comparison function (never Go's ==), so a non-comparable
// key type is fine.  Two empty queues are equal, a nil queue equals an
// empty queue, and Equal is symmetric on any pair that satisfies its
// precondition.
//
// Precondition: both queues must use consistent comparison functions (a
// nil queue trivially does).  Functions are not comparable, so this
// cannot be checked — two queues built by the same constructor over the
// same key type always satisfy it.
// Complexity is O(n log n).
func (pq *StablePQ[T]) Equal(other *StablePQ[T]) bool {
	if pq.Len() != other.Len() {
		return false
	}
	if pq.Len() == 0 {
		return true // both empty (either may be nil)
	}
	// A non-empty queue is necessarily constructor-built, so cmp is set.
	compare := pq.cmp
	a := pq.Slice()
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

// Clone returns a deep copy of the queue: a fresh queue with the same
// comparison function, the same insertion stamps on every element, and
// the same stamp counter — the inner heap is rebuilt with its (key,
// seq) items intact (the All snapshot pattern), and that pair orders
// totally, so the clone's pop order (FIFO among ties included) and the
// stamps its future Insert calls hand out are the receiver's.  Nothing
// is shared, so inserting into or popping from either queue afterwards
// never affects the other.  A nil queue clones to nil; an empty queue
// clones to a fresh empty queue; a zero-value queue clones to a fresh
// zero-value-shaped queue (still without an underlying heap, like the
// receiver).
// Complexity is O(n log n).
func (pq *StablePQ[T]) Clone() *StablePQ[T] {
	if pq == nil {
		return nil
	}
	dup := &StablePQ[T]{cmp: pq.cmp, stamp: pq.stamp}
	if pq.h != nil {
		dup.h = heap.NewHeapFunc(itemCmp(pq.cmp))
		for i := 0; i < pq.h.Len(); i++ {
			v, _ := pq.h.GetValue(i)
			dup.h.Push(v) // the (key, seq) item, stamps included
		}
	}
	return dup
}
