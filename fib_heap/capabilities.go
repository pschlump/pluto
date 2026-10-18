/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package fib_heap

import (
	"fmt"
	"strings"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 5) for the heap family — the §4.5
// menu only: materialization, rendering, equality and deep copy.  An
// indexed priority queue is a multiset with a pop order AND an index
// space, not a sequence and not a set, so there is deliberately no
// membership batch and no predicate bulk delete.  The observable
// materialization order is All's order — priority order, minimum value
// first (All drains a private snapshot), the order Pop would produce —
// so Slice and String use that order.  Equal compares the index→value
// bindings, not just the value multiset: this is the map-shaped
// exception within §4.5 (the queue's content is the association — swap
// the values of two present indices and the value multiset is unchanged
// but the content is not).  Clone rebuilds a fresh queue over the same
// index space with the same bindings.  Everything is additive, and every
// new method is a total function on the read side: a nil queue and the
// zero value behave as an empty queue, exactly like the reads in
// fib_heap.go.  None of the new methods panics on a nil queue — and
// none of them can panic on element comparison at all: equality goes
// through the stored comparison function, never through Go's ==, so a
// non-comparable value type is fine.

// Slice returns a fresh slice holding the queue's values in priority
// order — minimum first, the order All yields and Pop would produce.
// The values alone lose the index association; pair them with the
// indices by ranging over All directly.  Ties may appear in any order.
// The slice is a copy: it aliases none of the queue's storage, and
// queue operations after the call never change it.  A nil queue yields
// nil; an empty queue yields an empty (non-nil) slice.
// Complexity is O(n log n) — the snapshot drain All performs.
func (q *IndexFibonacciPQ[T]) Slice() []T {
	if q == nil {
		return nil
	}
	items := make([]T, 0, q.length)
	for _, v := range q.All() {
		items = append(items, v)
	}
	return items
}

// String renders the queue as its values formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in priority order
// (minimum first, All's order): "[a b c]".  A nil or empty queue renders
// as "[]".  For a custom per-element rendering use StringFunc.
// Complexity is O(n log n) plus the cost of formatting each element.
func (q *IndexFibonacciPQ[T]) String() string {
	return q.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the values are rendered in priority order and the formatted
// strings joined inside square brackets.  A nil format function renders
// as "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n log n) plus the cost of format.
func (q *IndexFibonacciPQ[T]) StringFunc(format func(T) string) string {
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

// Equal reports whether other holds the same index→value bindings as
// the receiver: same length, and for every index present in the
// receiver, the same index is present in other with a value that
// compares equal through the queue's comparison function (cmp(a, b) ==
// 0 — never Go's ==, so a non-comparable value type is fine).  This is
// the map-shaped exception within the §4.5 multiset rule: an indexed
// queue's content is the association, so two queues over the same VALUE
// multiset with the values swapped between indices are NOT equal.  The
// comparison never reorders either queue.  Two empty queues are equal
// (whatever their index spaces), a nil queue equals an empty queue, and
// Equal is symmetric on any pair that satisfies its precondition.
//
// Precondition: both queues must use consistent comparison functions (a
// nil queue trivially does).  Functions are not comparable, so this
// cannot be checked — two queues built by the same constructor over the
// same value type always satisfy it.
// Complexity is O(n).
func (q *IndexFibonacciPQ[T]) Equal(other *IndexFibonacciPQ[T]) bool {
	if q.Len() != other.Len() {
		return false
	}
	if q.Len() == 0 {
		return true // both empty (either may be nil)
	}
	// A non-empty queue is necessarily constructor-built, so cmp is set.
	// Equal lengths plus receiver ⊆ other means the bindings match
	// exactly — there is nothing left on other's side.
	compare := q.cmp
	for k, x := range q.nodes {
		if x == nil {
			continue
		}
		v, found := other.Value(k)
		if !found || compare(x.value, v) != 0 {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the queue: a fresh queue over the same
// index space with the same comparison function and the same index→value
// bindings.  The internal forest shape may differ from the receiver's
// (the nodes are re-linked in index order), which is unobservable —
// priority order depends only on the values.  No node is shared, so
// inserting into, changing or deleting from either queue afterwards
// never affects the other.  A nil queue clones to nil; an empty queue
// clones to a fresh empty queue over the same index space.
// Complexity is O(n).
func (q *IndexFibonacciPQ[T]) Clone() *IndexFibonacciPQ[T] {
	if q == nil {
		return nil
	}
	return q.clone() // the private snapshot builder All uses
}
