/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bag

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
// a bag holding [a b] is not equal to a bag holding [b a], where every
// set-shaped package compares membership — for a bag the order that
// counts is insertion order (the order All yields), so the SAME multiset
// added in a different order is a different bag.  Everything is
// additive, and every new method is a total function on the read side: a
// nil bag and the zero value behave as an empty bag, exactly like the
// reads in bag.go.  None of the new methods panics on a nil bag.

// Slice returns a fresh slice holding the bag's elements in insertion
// order — element 0 is the first element added, the order All yields.
// The slice is a copy: it aliases none of the bag's nodes, and bag
// operations after the call never change it (nor does changing it change
// the bag).  A nil bag yields nil; an empty bag yields an empty
// (non-nil) slice.
// Complexity is O(n).
func (b *Bag[T]) Slice() []T {
	if b == nil {
		return nil
	}
	items := make([]T, 0, b.length)
	for p := b.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return items
}

// String renders the bag as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in insertion order:
// "[a b c]" (the first element added first).  A nil or empty bag renders
// as "[]".  For a custom per-element rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (b *Bag[T]) String() string {
	return b.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in insertion order and the
// formatted strings joined inside square brackets.  A nil format
// function renders as "[]" (nothing is formatted, so a nil format is
// safe).
// Complexity is O(n) plus the cost of format.
func (b *Bag[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range b.Slice() {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same elements in the same order
// as the receiver, in insertion order — sequence equality, not set
// equality: a bag holding [a b] is NOT equal to a bag holding [b a],
// even though the two bags hold the same multiset.  (Sequences are the
// one pluto family whose Equal is order-sensitive; the set-shaped
// packages compare membership.)  Elements are compared pairwise with
// Go's == through interface conversion, so the element type must be
// comparable — a non-comparable element type (a slice, map or function)
// panics with Go's runtime comparison panic, but only when two elements
// are actually compared: bags of different lengths, and nil or empty
// bags, are answered without touching any element.  A nil bag equals
// only an empty bag.
// Complexity is O(n).
func (b *Bag[T]) Equal(other *Bag[T]) bool {
	if b.Len() != other.Len() {
		return false
	}
	if b.Len() == 0 {
		return true // both empty (either may be nil)
	}
	for p, op := b.head, other.head; p != nil; p, op = p.next, op.next {
		if any(p.data) != any(op.data) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the bag: a fresh bag holding the same
// elements in the same order, as a fresh node chain (no node and no
// link is shared with the receiver), so adding to or truncating either
// bag afterwards never affects the other.  A nil bag clones to nil; an
// empty bag clones to a fresh empty bag.
// Complexity is O(n).
func (b *Bag[T]) Clone() *Bag[T] {
	if b == nil {
		return nil
	}
	dup := &Bag[T]{}
	for p := b.head; p != nil; p = p.next {
		dup.Add(p.data) // Add appends at the tail: insertion order preserved
	}
	return dup
}
