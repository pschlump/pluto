/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package priority_queue

import (
	"fmt"
	"strings"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 5) — the §4.5 menu only:
// materialization, rendering, multiset equality and deep copy.  A
// priority queue is a multiset with a pop order, not a sequence and not
// a set, so there is deliberately no membership batch and no predicate
// bulk delete.
//
// priority_queue is the one §4.5 package whose observable order is
// SORTED: its All drains a private snapshot heap in priority order, and
// that — minimum element first, the order the JSON marshal writes too —
// is the order Slice and String use (the underlying heap's breadth-first
// backing array is not this queue's observable).  Equal can therefore
// compare the two materialized sequences directly: both come out already
// ordered by the comparator, so no re-sorting is needed and ties are safe
// by construction.  Everything is additive and every new method is a
// total function on the read side: a nil queue and the zero value behave
// as an empty queue.  Equality goes through the stored comparison
// function, never Go's ==, so a non-comparable element type is fine.

// Slice returns a fresh slice holding the queue's elements in priority
// order, minimum element first — the order All iterates in, the order
// the JSON marshal writes.  (This is the one §4.5 package where that
// order is sorted rather than the backing array order.)  The slice is a
// copy taken from All's private snapshot: it aliases none of the queue's
// storage, and queue operations after the call never change it.  A nil
// queue yields nil; an empty queue yields an empty (non-nil) slice.
// Complexity is O(n log n) — the same ordered snapshot All builds.
func (pq *PriorityQueue[T]) Slice() []T {
	if pq == nil {
		return nil
	}
	items := make([]T, 0, pq.Len())
	for v := range pq.All() {
		items = append(items, v)
	}
	return items
}

// String renders the queue as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in priority order,
// minimum element first: "[a b c]".  For a custom per-element rendering
// use StringFunc.  A nil or empty queue renders as "[]".
// Complexity is O(n log n) plus the cost of formatting each element.
func (pq *PriorityQueue[T]) String() string {
	return pq.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in priority order and the formatted
// strings joined inside square brackets.  A nil format function renders
// as "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n log n) plus the cost of format.
func (pq *PriorityQueue[T]) StringFunc(format func(T) string) string {
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

// Equal reports whether other holds the same multiset of elements as the
// receiver: same length and the same element sequence ordered by the
// comparator.  Two queues built from the same multiset in different
// insertion orders are equal.  Because both sides materialize in
// priority order — already sorted by the comparator — the comparison is
// a direct pairwise walk with no re-sorting; ties are safe (equal
// elements compare 0 with each other wherever they land).  The
// comparison is non-destructive: each side is materialized from All's
// private snapshot and the queues are never reordered.  Elements are
// compared through the queue's comparison function (never Go's ==), so a
// non-comparable element type is fine.  Two empty queues are equal, a
// nil queue equals an empty queue, and Equal is symmetric on any pair
// that satisfies its precondition.
//
// Precondition: both queues must use consistent comparison functions (a
// nil queue trivially does).  Functions are not comparable, so this
// cannot be checked — two queues built by the same constructor over the
// same element type always satisfy it.
// Complexity is O(n log n) — the snapshot builds dominate the walk.
func (pq *PriorityQueue[T]) Equal(other *PriorityQueue[T]) bool {
	if pq.Len() != other.Len() {
		return false
	}
	if pq.Len() == 0 {
		return true // both empty (either may be nil)
	}
	a := pq.Slice()    // priority order: sorted by the comparator
	b := other.Slice() // likewise — no re-sorting is needed
	compare := pq.cmp  // non-empty ⇒ built by a constructor
	for i := range a {
		if compare(a[i], b[i]) != 0 {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the queue: a fresh queue with the same
// comparison function and the same elements in the same underlying heap
// layout (the internal backing array is copied, which preserves the heap
// property exactly), so the clone's Slice, String and pop sequence are
// the receiver's.  No backing array is shared, so inserting into or
// popping from either queue afterwards never affects the other.  A nil
// queue clones to nil; an empty queue clones to a fresh empty queue; a
// zero-value queue clones to a fresh zero value (reads as empty, like
// the receiver).
// Complexity is O(n).
func (pq *PriorityQueue[T]) Clone() *PriorityQueue[T] {
	if pq == nil {
		return nil
	}
	dup := &PriorityQueue[T]{cmp: pq.cmp}
	if pq.h != nil {
		dup.h = pq.h.Clone() // copies the backing array — the exact layout
	}
	return dup
}
