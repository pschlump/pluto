/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bag_ts

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
//
// Lock discipline: the reads (Slice/StringFunc) snapshot the elements
// under the read lock (the same snapshot() All and Backward use) and
// compute afterwards without any lock held, so a caller format function
// may safely call back into the bag.  Equal snapshots each bag under its
// own lock and never holds two locks at once.  Clone snapshots under the
// read lock and rebuilds a fresh local bag without any lock held.

// Slice returns a fresh slice holding the bag's elements in insertion
// order — element 0 is the first element added, the order All yields.
// The snapshot is taken under the read lock, so this is safe to call
// concurrently with any bag operation.  The slice is a copy: it aliases
// none of the bag's nodes, and bag operations after the call never
// change it (nor does changing it change the bag).  A nil bag yields
// nil; an empty bag yields an empty (non-nil) slice.
// Complexity is O(n).
func (b *Bag[T]) Slice() []T {
	if b == nil {
		return nil
	}
	return b.snapshot()
}

// String renders the bag as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in insertion order:
// "[a b c]" (the first element added first).  The elements are
// snapshotted under the read lock and the formatting runs without the
// lock.  A nil or empty bag renders as "[]".  For a custom per-element
// rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (b *Bag[T]) String() string {
	return b.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in insertion order and the
// formatted strings joined inside square brackets.  The elements are
// snapshotted under the read lock and the format function runs without
// the lock, so it may safely call back into the bag.  A nil format
// function renders as "[]" (nothing is formatted, so a nil format is
// safe).
// Complexity is O(n) plus the cost of format.
func (b *Bag[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range b.Slice() { // snapshot taken, lock released
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
//
// Each bag is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines compare
// the same pair of bags in opposite directions.
// Complexity is O(n).
func (b *Bag[T]) Equal(other *Bag[T]) bool {
	if b.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if b.Len() == 0 {
		return true // both empty (either may be nil)
	}
	a := b.Slice()     // b's lock, released before other is touched
	c := other.Slice() // other's lock, one snapshot
	for i := range a { // no lock held: the copies are compared
		if any(a[i]) != any(c[i]) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the bag: a fresh bag holding the same
// elements in the same order, as a fresh node chain (no node and no
// link is shared with the receiver), so adding to or truncating either
// bag afterwards never affects the other.  The elements are snapshotted
// under the read lock and the rebuild runs on the local clone without
// any lock held.  A nil bag clones to nil; an empty bag clones to a
// fresh empty bag.
// Complexity is O(n).
func (b *Bag[T]) Clone() *Bag[T] {
	if b == nil {
		return nil
	}
	items := b.snapshot() // snapshot under the read lock
	dup := &Bag[T]{}
	for _, v := range items {
		dup.Add(v) // Add appends at the tail: insertion order preserved; dup is local, no contention
	}
	return dup
}
