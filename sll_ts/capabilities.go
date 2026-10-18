/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sll_ts

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
// a list holding [a b] is not equal to a list holding [b a], where every
// set-shaped package compares membership.  Everything is additive, and
// the four additions are total functions needing no errors — they fit
// the package's error-based API without disturbing it: a nil list and
// the zero value behave as an empty list, exactly like the reads in
// sll.go.  None of the new methods panics on a nil list.
//
// Lock discipline: the reads (Slice/StringFunc) snapshot the elements
// under the read lock and compute afterwards without any lock held, so a
// caller format function may safely call back into the list.  Equal
// snapshots each list under its own lock and never holds two locks at
// once.  Clone snapshots under the read lock and rebuilds a fresh local
// list.  No new Nl methods: the new methods are self-locking, not
// compounds.

// Slice returns a fresh slice holding the list's elements from head to
// tail — the order IterateOver and MarshalJSON yield (the head is
// element 0).  The snapshot is taken under the read lock, so this is
// safe to call concurrently with any list operation.  The slice is a
// copy: it aliases none of the list's nodes, and list operations after
// the call never change it (nor does changing it change the list).  A
// nil list yields nil; an empty list yields an empty (non-nil) slice.
// Complexity is O(n).
func (ns *Sll[T]) Slice() []T {
	return ns.snapshot()
}

// String renders the list as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, head to tail:
// "[a b c]" (the head first).  The elements are snapshotted under the
// read lock and the formatting runs without the lock.  A nil or empty
// list renders as "[]".  For a custom per-element rendering use
// StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (ns *Sll[T]) String() string {
	return ns.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered head to tail and the formatted
// strings joined inside square brackets.  The elements are snapshotted
// under the read lock and the format function runs without the lock, so
// it may safely call back into the list.  A nil format function renders
// as "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n) plus the cost of format.
func (ns *Sll[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range ns.Slice() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same elements in the same order
// as the receiver, head to tail — sequence equality, not set equality: a
// list holding [a b] is NOT equal to a list holding [b a].  (Sequences
// are the one pluto family whose Equal is order-sensitive; the
// set-shaped packages compare membership.)  Elements are compared
// pairwise with the list's equality function (the constructor's NewSll /
// NewSllFunc function), position by position.  Two empty lists are
// equal, a nil list equals an empty list, and Equal is symmetric on any
// pair that satisfies its precondition.
//
// Each list is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines compare
// the same pair of lists in opposite directions.
//
// Precondition: both lists must use consistent equality functions (a nil
// list trivially does).  Functions are not comparable, so this cannot be
// checked — two lists built by the same constructor over the same
// element type always satisfy it.
// Complexity is O(n).
func (ns *Sll[T]) Equal(other *Sll[T]) bool {
	if ns.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if ns.Len() == 0 {
		return true // both empty (either may be nil): eq is never consulted
	}
	a := ns.Slice()    // ns's lock, released before other is touched
	b := other.Slice() // other's lock, one snapshot
	eq := ns.eq        // eq is set only by the constructors, safe to read here
	for i := range a { // no lock held: the copies are compared
		if !eq(a[i], b[i]) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the list: a fresh list with the same
// equality function and the same elements in the same order, in fresh
// nodes.  No node is shared with the receiver, so inserting into or
// deleting from either list afterwards never affects the other.  The
// elements are snapshotted under the read lock (eq is set only by the
// constructors, safe to read unlocked) and the rebuild runs on the local
// clone without any lock held.  A nil list clones to nil; an empty list
// clones to a fresh empty list.
// Complexity is O(n).
func (ns *Sll[T]) Clone() *Sll[T] {
	if ns == nil {
		return nil
	}
	items := ns.snapshot() // snapshot under the read lock
	dup := &Sll[T]{eq: ns.eq}
	for _, v := range items {
		dup.InsertAfterTail(v) // dup is local: its own lock, no contention
	}
	return dup
}
