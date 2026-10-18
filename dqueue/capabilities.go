/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package dqueue

import (
	"fmt"
	"strings"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 4) for the sequence packages — the
// minimal §4.4 menu only: materialization, rendering, order-sensitive
// equality and deep copy.  Sequences are not sets, so there is
// deliberately no membership predicate, no batch insert/delete and no
// predicate bulk delete here — an O(n) membership scan over a sequence
// would be a footgun.  Equal is the one order-sensitive Equal in pluto:
// a deque holding [a b] is not equal to a deque holding [b a], where
// every set-shaped package compares membership.  Everything is additive,
// and every new method is a total function on the read side: a nil deque
// and the zero value behave as an empty deque, exactly like the reads in
// dqueue.go.  None of the new methods panics on a nil deque.

// Slice returns a fresh slice holding the deque's elements from the
// front to the back — the order All yields (index 0 of All is the
// front).  The slice is a copy: it aliases none of the deque's nodes,
// and deque operations after the call never change it (nor does
// changing it change the deque).  A nil deque yields nil; an empty deque
// yields an empty (non-nil) slice.
// Complexity is O(n).
func (q *Deque[T]) Slice() []T {
	if q == nil {
		return nil
	}
	items := make([]T, 0, q.length)
	for p := q.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return items
}

// String renders the deque as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, front to back:
// "[a b c]".  A nil or empty deque renders as "[]".  For a custom
// per-element rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (q *Deque[T]) String() string {
	return q.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered front to back and the formatted
// strings joined inside square brackets.  A nil format function renders
// as "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n) plus the cost of format.
func (q *Deque[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range q.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same elements in the same order
// as the receiver, front to back — sequence equality, not set equality:
// a deque holding [a b] is NOT equal to a deque holding [b a].
// (Sequences are the one pluto family whose Equal is order-sensitive;
// the set-shaped packages compare membership.)  Elements are compared
// pairwise with Go's == through interface conversion, so the element
// type must be comparable — a non-comparable element type (a slice, map
// or function) panics with Go's runtime comparison panic, but only when
// two elements are actually compared: deques of different lengths, and
// nil or empty deques, are answered without touching any element.  A nil
// deque equals only an empty deque.
// Complexity is O(n).
func (q *Deque[T]) Equal(other *Deque[T]) bool {
	if q.Len() != other.Len() {
		return false
	}
	if q.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for p, op := q.head, other.head; p != nil; p, op = p.next, op.next {
		if any(p.data) != any(op.data) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the deque: a fresh deque holding the same
// elements in the same order, as a fresh node chain (no node and no
// link is shared with the receiver), so pushing to or popping from
// either deque afterwards never affects the other.  A nil deque clones
// to nil; an empty deque clones to a fresh empty deque.
// Complexity is O(n).
func (q *Deque[T]) Clone() *Deque[T] {
	if q == nil {
		return nil
	}
	dup := &Deque[T]{}
	for p := q.head; p != nil; p = p.next {
		dup.PushBack(p.data)
	}
	return dup
}
