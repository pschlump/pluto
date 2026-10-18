/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package quicklist_ts

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
// a list holding [a b] is not equal to a list holding [b a], where
// every set-shaped package compares membership.  Everything is additive,
// and every new method is a total function on the read side: a nil list
// and the zero value behave as an empty list, exactly like the reads in
// quicklist.go.  None of the new methods panics on a nil list.
//
// Lock discipline: Slice and the String reads are At-shaped reads — with
// compression enabled the walk materializes the segments it touches,
// which mutates, so they take the WRITE lock exactly as At and snapshot
// do (a read lock would race with itself).  The caller format function
// of StringFunc runs on the snapshot without any lock held, so it may
// safely call back into the list.  Equal snapshots each list under its
// own lock and never holds two locks at once.  Clone holds the write
// lock while the source walk materializes, and builds the local copy
// without any lock held on it.  No new Nl methods: the new methods are
// self-locking, not compounds.

// Slice returns a fresh slice holding the list's elements head to tail —
// the order All iterates (index 0 is the head).  It is an At-shaped
// read: with compression enabled the snapshot walk materializes the
// segments it touches, so it takes the write lock exactly as At does,
// and is safe to call concurrently with any list operation.  The slice
// is a copy: it aliases none of the list's storage, and list operations
// after the call never change it (nor does changing it change the
// list).  A nil list yields nil; an empty list yields an empty
// (non-nil) slice.
// Complexity is O(S + n), S = number of segments.
func (q *QuickList[T]) Slice() []T {
	return q.snapshot() // nil-guard and write lock inside
}

// String renders the list as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, head to tail:
// "[a b c]".  The elements are snapshotted under the write lock
// (At-shaped) and the formatting runs without the lock.  A nil or empty
// list renders as "[]".  For a custom per-element rendering use
// StringFunc.
// Complexity is O(S + n) plus the cost of formatting each element.
func (q *QuickList[T]) String() string {
	return q.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered head to tail and the formatted
// strings joined inside square brackets.  The elements are snapshotted
// under the write lock (At-shaped) and the format function runs without
// the lock, so it may safely call back into the list.  A nil format
// function renders as "[]" (nothing is formatted, so a nil format is
// safe).
// Complexity is O(S + n) plus the cost of format.
func (q *QuickList[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range q.Slice() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same elements in the same order
// as the receiver, head to tail — sequence equality, not set equality:
// a list holding [a b] is NOT equal to a list holding [b a].
// (Sequences are the one pluto family whose Equal is order-sensitive;
// the set-shaped packages compare membership.)  Elements are compared
// pairwise with Go's == through interface conversion, so the element
// type must be comparable — a non-comparable element type (a slice, map
// or function) panics with Go's runtime comparison panic, but only when
// two elements are actually compared: lists of different lengths, and
// nil or empty lists, are answered without touching any element.  A nil
// list equals only an empty list.
//
// Each list is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines compare
// the same pair of lists in opposite directions.
// Complexity is O(S + n).
func (q *QuickList[T]) Equal(other *QuickList[T]) bool {
	if q.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if q.Len() == 0 {
		return true // both empty (either may be nil)
	}
	a := q.Slice()     // q's write lock (At-shaped), released before other
	b := other.Slice() // other's write lock, one snapshot
	for i := range a { // no lock held: the copies are compared
		if any(a[i]) != any(b[i]) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the list: a fresh list holding the same
// elements in the same order, with the receiver's whole configuration —
// fill target, byte cap, compression codec, depth and enc/dec — carried
// over, so the clone segments and compresses exactly like the receiver.
// No segment or backing array is shared with the receiver, so pushing to
// or popping from either list afterwards never affects the other.  The
// source walk is At-shaped (it materializes compressed segments), so the
// write lock is held for it; the copy is a fresh local list whose own
// zero-value lock is ready.  A nil list clones to nil; an empty list
// clones to a fresh empty list.
// Complexity is O(S + n).
func (q *QuickList[T]) Clone() *QuickList[T] {
	if q == nil {
		return nil
	}
	q.lock.Lock() // At-shaped: the source walk materializes segments
	defer q.lock.Unlock()
	dup := &QuickList[T]{}
	dup.inner = *q.inner.Clone()
	return dup
}
