/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binomial_queue

import (
	"fmt"
	"slices"
	"strings"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 5) for the heap family — the §4.5
// menu only: materialization, rendering, multiset equality and deep copy.
// A binomial queue is a multiset with a pop order, not a sequence and
// not a set, so there is deliberately no membership batch and no
// predicate bulk delete.  The observable materialization order is All's
// order — internal forest order, the trees in increasing degree order
// and each tree in pre-order, the order the JSON marshal uses (the
// note/implement-json.md precedent) — so Slice and String use that
// order; repeatedly calling DeleteMin remains the way to consume the
// queue in sorted order.  Equal compares multisets, not forest shapes:
// two queues over the same multiset hold different forests (the shape
// depends on the insertion and merge history), so Equal sorts a copy of
// each side with the stored comparator and compares the sorted value
// sequences — non-destructively, the receiver is never reordered.
// Clone rebuilds by re-insertion, so the clone's forest shape may differ
// from the receiver's — only the multiset and the sorted drain order are
// observable, and both survive.  Everything is additive, and every new
// method is a total function on the read side: a nil queue and the zero
// value behave as an empty queue, exactly like the reads in
// binomial_queue.go.  None of the new methods panics on a nil queue —
// and none of them can panic on element comparison at all: equality goes
// through the stored comparison function, never through Go's ==, so a
// non-comparable element type is fine.

// Slice returns a fresh slice holding the queue's elements in internal
// forest order — the trees in increasing degree order, each tree in
// pre-order — the order All yields and the JSON marshal uses.  This is
// NOT sorted order; DeleteMin is the sorted drain.  The slice is a copy:
// it aliases none of the queue's storage, and queue operations after the
// call never change it.  A nil queue yields nil; an empty queue yields
// an empty (non-nil) slice.
// Complexity is O(n).
func (q *BinomialQueue[T]) Slice() []T {
	if q == nil {
		return nil
	}
	items := make([]T, 0, q.length)
	for _, v := range q.All() {
		items = append(items, v)
	}
	return items
}

// String renders the queue as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in internal forest
// order (All's order, not sorted order): "[a b c]".  A nil or empty
// queue renders as "[]".  For a custom per-element rendering use
// StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (q *BinomialQueue[T]) String() string {
	return q.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in internal forest order and the
// formatted strings joined inside square brackets.  A nil format
// function renders as "[]" (nothing is formatted, so a nil format is
// safe).
// Complexity is O(n) plus the cost of format.
func (q *BinomialQueue[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range q.Slice() {
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
// queues built from the same multiset in different insertion orders —
// or by different merge histories — hold different forests but are
// equal; ties are safe (the sorted sequences agree element for element
// exactly when the multisets do).  The comparison is non-destructive:
// each side is copied and the copy sorted — the receiver and other are
// never reordered.  Elements are compared through the queue's
// comparison function (never Go's ==), so a non-comparable element type
// is fine.  Two empty queues are equal, a nil queue equals an empty
// queue, and Equal is symmetric on any pair that satisfies its
// precondition.
//
// Precondition: both queues must use consistent comparison functions (a
// nil queue trivially does).  Functions are not comparable, so this
// cannot be checked — two queues built by the same constructor over the
// same element type always satisfy it.
// Complexity is O(n log n).
func (q *BinomialQueue[T]) Equal(other *BinomialQueue[T]) bool {
	if q.Len() != other.Len() {
		return false
	}
	if q.Len() == 0 {
		return true // both empty (either may be nil)
	}
	// A non-empty queue is necessarily constructor-built, so cmp is set.
	compare := q.cmp
	a := q.Slice()
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
// comparison function and the same elements, rebuilt by re-insertion.
// The clone's forest shape may differ from the receiver's (the shape
// depends on the insertion history), which is unobservable — the
// multiset and the sorted DeleteMin drain are the content, and both are
// the receiver's.  No node is shared, so inserting into, merging into or
// draining either queue afterwards never affects the other.  A nil
// queue clones to nil; an empty queue clones to a fresh empty queue; a
// zero-value queue clones to a fresh zero-value-shaped queue (still
// without a comparison function, like the receiver — but still usable
// as a Merge target, which adopts the operand's comparison function).
// Complexity is O(n) amortized — n inserts at O(1) amortized each.
func (q *BinomialQueue[T]) Clone() *BinomialQueue[T] {
	if q == nil {
		return nil
	}
	dup := &BinomialQueue[T]{cmp: q.cmp}
	for _, v := range q.Slice() {
		dup.Insert(v) // cmp is non-nil whenever there is anything to insert
	}
	return dup
}
