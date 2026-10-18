/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package stack_sll_ts

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
// a stack holding [a b] is not equal to a stack holding [b a], where
// every set-shaped package compares membership.  Everything is additive,
// and every new method is a total function on the read side: a nil stack
// and the zero value behave as an empty stack, exactly like the reads in
// stack.go.  None of the new methods panics on a nil stack.
//
// Lock discipline: the reads (Slice/StringFunc) snapshot the elements
// under the read lock (the same snapshot() All and Backward use) and
// compute afterwards without any lock held, so a caller format function
// may safely call back into the stack.  Equal snapshots each stack under
// its own lock and never holds two locks at once.  Clone snapshots under
// the read lock and rebuilds a fresh local stack without any lock held.

// Slice returns a fresh slice holding the stack's elements top down —
// the top (most recently pushed) element first, the order All yields
// (index 0 of All is the top).  The snapshot is taken under the read
// lock, so this is safe to call concurrently with any stack operation.
// The slice is a copy: it aliases none of the stack's nodes, and stack
// operations after the call never change it (nor does changing it change
// the stack).  A nil stack yields nil; an empty stack yields an empty
// (non-nil) slice.
// Complexity is O(n).
func (ns *Stack[T]) Slice() []T {
	return ns.snapshot()
}

// String renders the stack as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, top down: "[a b c]"
// (the top element first, the order All yields).  The elements are
// snapshotted under the read lock and the formatting runs without the
// lock.  A nil or empty stack renders as "[]".  For a custom per-element
// rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (ns *Stack[T]) String() string {
	return ns.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered top down and the formatted strings
// joined inside square brackets.  The elements are snapshotted under the
// read lock and the format function runs without the lock, so it may
// safely call back into the stack.  A nil format function renders as
// "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n) plus the cost of format.
func (ns *Stack[T]) StringFunc(format func(T) string) string {
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
// as the receiver, top down — sequence equality, not set equality: a
// stack holding [a b] (b on top) is NOT equal to a stack holding [b a]
// (a on top).  (Sequences are the one pluto family whose Equal is
// order-sensitive; the set-shaped packages compare membership.)
// Elements are compared pairwise with Go's == through interface
// conversion, so the element type must be comparable — a non-comparable
// element type (a slice, map or function) panics with Go's runtime
// comparison panic, but only when two elements are actually compared:
// stacks of different lengths, and nil or empty stacks, are answered
// without touching any element.  A nil stack equals only an empty stack.
//
// Each stack is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines compare
// the same pair of stacks in opposite directions.
// Complexity is O(n).
func (ns *Stack[T]) Equal(other *Stack[T]) bool {
	if ns.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if ns.Len() == 0 {
		return true // both empty (either may be nil)
	}
	a := ns.Slice()    // ns's lock, released before other is touched
	b := other.Slice() // other's lock, one snapshot
	for i := range a { // no lock held: the copies are compared
		if any(a[i]) != any(b[i]) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the stack: a fresh stack holding the same
// elements in the same order, as a fresh node chain (no node and no link
// is shared with the receiver), so pushing to or popping from either
// stack afterwards never affects the other.  The elements are
// snapshotted under the read lock and the rebuild runs on the local
// clone without any lock held.  A nil stack clones to nil; an empty
// stack clones to a fresh empty stack.
// Complexity is O(n).
func (ns *Stack[T]) Clone() *Stack[T] {
	if ns == nil {
		return nil
	}
	items := ns.snapshot() // top first
	dup := &Stack[T]{}
	for i := len(items) - 1; i >= 0; i-- {
		dup.Push(items[i]) // bottom first, so the last push is the top again
	}
	return dup
}
