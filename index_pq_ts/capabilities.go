/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package index_pq_ts

import (
	"fmt"
	"slices"
	"strings"
)

// This file holds the convenience capabilities (the hashicorp/go-set
// translation, set-capabilities phase 5) — the §4.5 menu only:
// materialization, rendering, equality and deep copy.  There is
// deliberately no membership batch and no predicate bulk delete.
//
// An IndexPQ is MAP-SHAPED, not multiset-shaped, and the capabilities
// follow that: the content is the set of index→value bindings, so Equal
// compares bindings — swapping the values of two indices changes the
// content even though the value multiset is unchanged, and such queues
// are NOT equal.  Slice materializes the values (the keys bound to the
// indices) in backing order — the breadth-first order of the index heap
// pq, not priority order; the (index, value) pairs in priority order are
// All's.  Clone rebuilds the bindings pair by pair.  Everything is
// additive and every new method is a total function on the read side: a
// nil queue and the zero value behave as an empty queue.  Values are
// compared through the stored comparison function, never Go's ==, so a
// non-comparable value type is fine.
//
// Lock discipline: Slice snapshots the values under the read lock;
// everything after the snapshot — the StringFunc format callback,
// Equal's pairwise walk, Clone's local rebuild — runs without any lock
// held, so a caller format function may safely call back into the queue.
// Equal snapshots the receiver under its own read lock and then touches
// other one Value call at a time — never two locks at once, so two
// queues can be compared from concurrent goroutines in opposite
// directions without deadlock.  Clone snapshots under the read lock and
// rebuilds a fresh local queue; cmp is set only by the constructors, so
// reading it to copy it into the clone is safe.  No new Nl methods: the
// capabilities are self-locking, not compounds.

// Slice returns a fresh slice holding the queue's values in backing
// order — the values bound to the indices, in the breadth-first order of
// the internal index heap.  This is NOT priority order: All yields the
// (index, value) pairs sorted by value, Pop drains that order.  The
// snapshot is taken under the read lock, so this is safe to call
// concurrently with any queue operation, and the slice is a copy: it
// aliases none of the queue's storage.  A nil queue yields nil; an empty
// queue yields an empty (non-nil) slice.
// Complexity is O(n).
func (q *IndexPQ[T]) Slice() []T {
	if q == nil {
		return nil
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	items := make([]T, 0, q.length)
	for _, k := range q.pq {
		items = append(items, q.vals[k])
	}
	return items
}

// String renders the queue as its values formatted with fmt.Sprint and
// joined with single spaces inside square brackets, in backing order —
// the breadth-first index-heap order, NOT All's priority order:
// "[a b c]".  The values are snapshotted under the read lock and the
// formatting runs without it.  A nil or empty queue renders as "[]".
// For a custom per-element rendering use StringFunc.
// Complexity is O(n) plus the cost of formatting each value.
func (q *IndexPQ[T]) String() string {
	return q.StringFunc(func(v T) string { return fmt.Sprint(v) })
}

// StringFunc is String with a caller supplied per-element format
// function; the values are rendered in backing order and the formatted
// strings joined inside square brackets.  The values are snapshotted
// under the read lock and the format function runs without it, so it may
// safely call back into the queue.  A nil format function renders as
// "[]" (nothing is formatted, so a nil format is safe).
// Complexity is O(n) plus the cost of format.
func (q *IndexPQ[T]) StringFunc(format func(T) string) string {
	if format == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range q.Slice() { // snapshot taken, lock released
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprint(&sb, format(v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// Equal reports whether other holds the same index→value bindings as
// the receiver: same length, and every index present in the receiver is
// present in other with a value that compares equal (through the
// comparison function, never Go's ==).  This is BINDING equality, not
// multiset equality: two queues whose values are swapped between indices
// hold the same value multiset but different content and are NOT equal.
// The index space itself (the constructor's n) is capacity, not content
// — queues of different sizes that hold the same bindings are equal.
// Duplicate values are fine: bindings compare per index, so ties cannot
// mismatch.  The comparison is non-destructive (reads only).  Two empty
// queues are equal, a nil queue equals an empty queue, and Equal is
// symmetric on any pair that satisfies its precondition.
//
// The receiver is snapshotted under its own read lock and other is
// touched one Value call at a time — never two locks at once, so Equal
// never deadlocks, not even when two goroutines compare the same pair of
// queues in opposite directions.
//
// Precondition: both queues must use consistent comparison functions (a
// nil queue trivially does).  Functions are not comparable, so this
// cannot be checked — two queues built by the same constructor over the
// same value type always satisfy it.
// Complexity is O(n).
func (q *IndexPQ[T]) Equal(other *IndexPQ[T]) bool {
	if q.Len() != other.Len() { // each Len under its own lock, never both
		return false
	}
	if q.Len() == 0 {
		return true // both empty (either may be nil)
	}
	q.lock.RLock()
	compare := q.cmp // set only by the constructors; non-empty ⇒ set
	idx := slices.Clone(q.pq)
	vals := make([]T, len(idx))
	for i, k := range idx {
		vals[i] = q.vals[k]
	}
	q.lock.RUnlock()
	for i, k := range idx { // no lock held on q; other is read one Value at a time
		v, found := other.Value(k)
		if !found || compare(vals[i], v) != 0 {
			return false
		}
	}
	return true
}

// Clone returns a deep copy of the queue: a fresh queue over the same
// index space with the same comparison function and the same
// index→value bindings, rebuilt pair by pair.  No backing array is
// shared, so inserting into, changing or deleting from either queue
// afterwards never affects the other.  The bindings are snapshotted
// under the read lock and the rebuild runs on the local clone without
// any lock held.  A nil queue clones to nil; an empty queue clones to a
// fresh empty queue over the same index space; a zero-value queue clones
// to a fresh zero value (reads as empty, like the receiver).
// Complexity is O(n log n) for the per-pair rebuild.
func (q *IndexPQ[T]) Clone() *IndexPQ[T] {
	if q == nil {
		return nil
	}
	q.lock.RLock()
	cmp := q.cmp // set only by the constructors, safe to read (copied below)
	n := q.n
	idx := slices.Clone(q.pq)
	vals := make([]T, len(idx))
	for i, k := range idx {
		vals[i] = q.vals[k]
	}
	q.lock.RUnlock()
	if cmp == nil {
		return &IndexPQ[T]{} // a zero-value queue clones to a fresh zero value
	}
	dup := NewIndexPQFunc(n, cmp)
	for i, k := range idx { // dup is local: no lock is held on q
		dup.Insert(k, vals[i])
	}
	return dup
}
