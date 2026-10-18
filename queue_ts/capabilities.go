/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package queue_ts

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
// a queue holding [a b] is not equal to a queue holding [b a], where
// every set-shaped package compares membership.  Everything is additive,
// and every new method is a total function on the read side: a nil queue
// and the zero value behave as an empty queue, exactly like the reads in
// queue.go.  None of the new methods panics on a nil queue.
//
// Lock discipline: the reads (Slice/StringFunc) snapshot the elements
// under the read lock and compute afterwards without any lock held, so a
// caller format function may safely call back into the queue.  Equal
// snapshots each queue under its own lock and never holds two locks at
// once.  Clone copies under the read lock into a fresh local queue.  No
// new Nl methods: the new methods are self-locking, not compounds.

// Slice returns a fresh slice holding the queue's elements in FIFO order
// — head (the next element to dequeue) first, the order All yields.  The
// snapshot is taken under the read lock, so this is safe to call
// concurrently with any queue operation.  The slice is a copy: it
// aliases none of the queue's storage, and queue operations after the
// call never change it (nor does changing it change the queue).  A nil
// queue yields nil; an empty queue yields an empty (non-nil) slice.
// Complexity is O(n).
func (q *Queue[T]) Slice() []T {
	if q == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	items := make([]T, 0, len(q.data))
	items = append(items, q.data...)
	return items
}

// String renders the queue as its elements formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in FIFO order:
// "[a b c]" (head first).  The elements are snapshotted under the read
// lock and the formatting runs without the lock.  A nil or empty queue
// renders as "[]".  For a custom per-element rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each element.
func (q *Queue[T]) String() string {
	return q.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the elements are rendered in FIFO order and the formatted
// strings joined inside square brackets.  The elements are snapshotted
// under the read lock and the format function runs without the lock, so
// it may safely call back into the queue.  A nil format function renders
// as "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n) plus the cost of format.
func (q *Queue[T]) StringFunc(format func(T) string) string {
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
// as the receiver — sequence equality, not set equality: a queue holding
// [a b] is NOT equal to a queue holding [b a].  (Sequences are the one
// pluto family whose Equal is order-sensitive; the set-shaped packages
// compare membership.)  Elements are compared pairwise with Go's ==
// through interface conversion, so the element type must be comparable —
// a non-comparable element type (a slice, map or function) panics with
// Go's runtime comparison panic, but only when two elements are actually
// compared: queues of different lengths, and nil or empty queues, are
// answered without touching any element.  A nil queue equals only an
// empty queue.
//
// Each queue is touched under its own lock only, never two locks at
// once, so Equal never deadlocks — not even when two goroutines compare
// the same pair of queues in opposite directions.
// Complexity is O(n).
func (q *Queue[T]) Equal(other *Queue[T]) bool {
	if q.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if q.Len() == 0 {
		return true // both empty (either may be nil)
	}
	a := q.Slice()     // q's lock, released before other is touched
	b := other.Slice() // other's lock, one snapshot
	for i := range a { // no lock held: the copies are compared
		if any(a[i]) != any(b[i]) {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the queue: a fresh queue holding the same
// elements in the same order, in its own backing storage.  No memory is
// shared with the receiver, so pushing to or popping from either queue
// afterwards never affects the other.  The elements are copied under the
// read lock; the result is a fresh local queue whose own zero-value lock
// is ready.  A nil queue clones to nil; an empty queue clones to a fresh
// empty queue.
// Complexity is O(n).
func (q *Queue[T]) Clone() *Queue[T] {
	if q == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	dup := &Queue[T]{}
	dup.data = make([]T, len(q.data))
	copy(dup.data, q.data)
	return dup // a fresh local queue; its own zero-value lock is ready
}
