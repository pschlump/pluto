/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package index_pq implements a generic indexed priority queue — a
// min-first priority queue where the client owns stable indices 0..n-1
// and can change or delete the value associated with any index
// (Sedgwick's IndexMinPQ, Algorithms §2.6).  Peek and Pop always return
// the minimum value together with its index.
//
// The queue supports the following operations:
//
//	Insert(k, v) — add index k with value v (replaces if present).	O(log n)
//	Change(k, v) — replace the value of a present index.			O(log n)
//	Delete(k)    — remove index k.									O(log n)
//	Contains(k)  — is index k in the queue.							O(1)
//	Value(k)     — the value associated with index k.				O(1)
//	Peek         — the minimum index and value, without removing.	O(1)
//	Pop          — remove and return the minimum index and value.	O(log n)
//	All          — iterate (index, value) in priority order.		O(n log n)
//	Len / IsEmpty — size queries.									O(1)
//	Truncate     — remove all values.								O(n)
//
// The queue is self-contained rather than a composition over
// pluto/heap: an indexed priority queue needs the inverse position map
// (qp) maintained inside the heap's swim/sink, which a plain heap does
// not expose.
//
// The element type never implements an interface: queues of naturally
// ordered value types (all integers, floats and strings) are created
// with NewIndexPQ, which orders by the built-in < and > operators;
// queues of any other type are created with NewIndexPQFunc, which takes
// a caller supplied comparison function.  A reversed comparison turns
// the min-first queue into a max-first queue.
//
// A nil *IndexPQ and the zero value both behave as an empty queue for
// every operation except Insert and Change.  The package panics in
// exactly four situations, all programmer errors:
//
//	NewIndexPQFunc(n, nil)         — nil comparison function, caught at construction.
//	NewIndexPQ/NewIndexPQFunc n<1  — no usable index space.
//	Insert/Change on a nil queue   — a nil queue cannot store a value.
//	Insert/Change on a zero-value queue — no comparison function; the message names the constructors.
//
// Every other operation — including out-of-range indices — tolerates a
// nil queue and the zero value as an empty queue and reports false.
//
// The queue is NOT safe for concurrent use; the mutex-guarded twin
// index_pq_ts has the same interface.
package index_pq

import (
	"cmp"
	"iter"
	"slices"
)

// IndexPQ is an indexed priority queue over the indices 0..n-1 with
// values of type T ordered by its comparison function.  Use NewIndexPQ
// for naturally ordered value types (numbers, strings) or
// NewIndexPQFunc for a caller supplied comparison function.  The zero
// value is an empty queue for reads, but Insert and Change on it panic
// — create queues with the constructors.
type IndexPQ[T any] struct {
	n      int   // maximum number of indices: the valid indices are 0..n-1
	length int   // number of indices currently in the queue
	pq     []int // heap of indices in breadth-first order; pq[0] is the minimum
	qp     []int // qp[k] is the heap position of index k, -1 if k is absent
	vals   []T   // vals[k] is the value associated with index k

	// cmp orders two values: negative if a sorts before b, 0 if the two
	// are duplicates, positive if a sorts after b.  It is set by the
	// constructors and is the only thing that knows how to order T — T
	// itself never has to implement an interface.
	cmp func(a, b T) int
}

// Compare returns -1 if a < b, +1 if a > b and 0 if a == b, using the
// built-in ordering of any cmp.Ordered type (all integers, floats and
// strings).  It is the comparison function installed by NewIndexPQ and
// is handy for building custom comparison functions — including reversed
// ones, which turn the min-first queue into a max-first queue.
func Compare[T cmp.Ordered](a, b T) int {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	default:
		return 0
	}
}

// NewIndexPQ creates a new empty indexed priority queue over the indices
// 0..n-1 for any naturally ordered value type (all integers, floats and
// strings — cmp.Ordered).  Ordering uses the built-in < and > operators
// of T; no interface and no boxing is involved.
// It panics if n < 1.
// Complexity is O(n) for the index maps.
func NewIndexPQ[T cmp.Ordered](n int) *IndexPQ[T] {
	return NewIndexPQFunc(n, Compare[T])
}

// NewIndexPQFunc creates a new empty indexed priority queue over the
// indices 0..n-1 that orders values with the caller supplied comparison
// function fx.  fx must return a negative value if a sorts before b, 0
// if the two are duplicates and a positive value if a sorts after b, and
// must order values consistently.  A reversed comparison turns the
// min-first queue into a max-first queue.
// It panics if fx is nil or if n < 1.
// Complexity is O(n) for the index maps.
func NewIndexPQFunc[T any](n int, fx func(a, b T) int) *IndexPQ[T] {
	if fx == nil {
		panic("index_pq: NewIndexPQFunc called with a nil comparison function")
	}
	if n < 1 {
		panic("index_pq: NewIndexPQ/NewIndexPQFunc called with n < 1 (no usable index space)")
	}
	qp := make([]int, n)
	for i := range qp {
		qp[i] = -1
	}
	return &IndexPQ[T]{
		n:    n,
		pq:   make([]int, 0, n),
		qp:   qp,
		vals: make([]T, n),
		cmp:  fx,
	}
}

// compare orders a and b, guarding against a zero-value queue that was
// not created by one of the constructors.
func (q *IndexPQ[T]) compare(a, b T) int {
	if q.cmp == nil {
		panic("index_pq: no comparison function (create the queue with NewIndexPQ or NewIndexPQFunc)")
	}
	return q.cmp(a, b)
}

// less reports whether the index at heap position i orders before the
// index at heap position j.
func (q *IndexPQ[T]) less(i, j int) bool {
	return q.compare(q.vals[q.pq[i]], q.vals[q.pq[j]]) < 0
}

// swap exchanges the indices at heap positions i and j, keeping the
// inverse position map qp up to date.
func (q *IndexPQ[T]) swap(i, j int) {
	q.pq[i], q.pq[j] = q.pq[j], q.pq[i]
	q.qp[q.pq[i]] = i
	q.qp[q.pq[j]] = j
}

// swim moves the index at heap position j up towards the root until the
// heap property is restored.
func (q *IndexPQ[T]) swim(j int) {
	for {
		i := (j - 1) / 2 // pick the parent
		if i == j || !q.less(j, i) {
			break
		}
		q.swap(i, j)
		j = i
	}
}

// sink moves the index at heap position i0 down towards the leaves until
// the heap property is restored, treating n as the effective size of the
// heap (only positions < n are considered).  It reports whether the
// index moved.
func (q *IndexPQ[T]) sink(i0, n int) (moved bool) {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n {
			break
		}
		j := j1 // choose the left child
		if j2 := j1 + 1; j2 < n && q.less(j2, j1) {
			j = j2 // choose the right child
		}
		if !q.less(j, i) {
			break
		}
		q.swap(i, j)
		i = j
	}
	return i > i0
}

// fix re-establishes the heap ordering after the value of the index at
// heap position pos has been replaced.
func (q *IndexPQ[T]) fix(pos int) {
	if !q.sink(pos, q.length) {
		q.swim(pos)
	}
}

// deletePos removes the index at heap position pos and returns it along
// with its value.  The vacated value slot is zeroed so the GC can
// reclaim the removed value.
func (q *IndexPQ[T]) deletePos(pos int) (k int, v T) {
	k = q.pq[pos]
	v = q.vals[k]
	n := q.length - 1
	if pos != n {
		q.swap(pos, n) // move the last index into the gap
		q.length--
		if !q.sink(pos, q.length) {
			q.swim(pos)
		}
	} else {
		q.length--
	}
	q.pq = q.pq[:q.length]
	q.qp[k] = -1
	var zero T
	q.vals[k] = zero // zero the slot so the GC can reclaim the removed value
	return k, v
}

// Insert adds index k to the queue with value v.  If k is already in the
// queue its value is replaced (and the heap re-ordered) — the
// duplicates-replace convention, cheaper than a Delete followed by an
// Insert.  It reports false and does nothing if k is out of range.
// Insert panics on a nil queue or on a zero-value queue (no comparison
// function); these are two of the package's four panics — every other
// operation treats both as an empty queue.
// Complexity is O(log n).
func (q *IndexPQ[T]) Insert(k int, v T) bool {
	if q == nil {
		panic("index_pq: Insert called on a nil queue")
	}
	if q.cmp == nil {
		panic("index_pq: Insert called on a queue with no comparison function (create the queue with NewIndexPQ or NewIndexPQFunc)")
	}
	if k < 0 || k >= q.n {
		return false
	}
	if q.qp[k] != -1 {
		q.vals[k] = v
		q.fix(q.qp[k])
		return true
	}
	q.qp[k] = q.length
	q.vals[k] = v
	q.pq = append(q.pq, k)
	q.length++
	q.swim(q.length - 1)
	return true
}

// Change replaces the value of index k with v and re-establishes the
// heap ordering — the decrease-key (or increase-key) operation.  It
// reports false and does nothing if k is out of range or not in the
// queue.
// Change panics on a nil queue or on a zero-value queue (no comparison
// function), exactly like Insert.
// Complexity is O(log n).
func (q *IndexPQ[T]) Change(k int, v T) bool {
	if q == nil {
		panic("index_pq: Change called on a nil queue")
	}
	if q.cmp == nil {
		panic("index_pq: Change called on a queue with no comparison function (create the queue with NewIndexPQ or NewIndexPQFunc)")
	}
	if k < 0 || k >= q.n || q.qp[k] == -1 {
		return false
	}
	q.vals[k] = v
	q.fix(q.qp[k])
	return true
}

// Delete removes index k from the queue.  It reports false if k is out
// of range or not in the queue.
// Complexity is O(log n).
func (q *IndexPQ[T]) Delete(k int) bool {
	if q == nil || k < 0 || k >= q.n || q.qp[k] == -1 {
		return false
	}
	q.deletePos(q.qp[k])
	return true
}

// Contains reports whether index k is in the queue.
// Complexity is O(1).
func (q *IndexPQ[T]) Contains(k int) bool {
	return q != nil && k >= 0 && k < q.n && q.qp[k] != -1
}

// Value returns the value associated with index k, or false if k is out
// of range or not in the queue.
//
// The value is returned by value; it does not alias the queue's
// internals.
// Complexity is O(1).
func (q *IndexPQ[T]) Value(k int) (v T, found bool) {
	if q == nil || k < 0 || k >= q.n || q.qp[k] == -1 {
		return
	}
	return q.vals[k], true
}

// Peek returns the index and value of the minimum without removing it,
// or false if the queue is empty.
// Complexity is O(1).
func (q *IndexPQ[T]) Peek() (k int, v T, found bool) {
	if q == nil || q.length == 0 {
		return
	}
	k = q.pq[0]
	return k, q.vals[k], true
}

// Pop removes and returns the index and value of the minimum, or false
// if the queue is empty.
//
// Both are returned by value; the queue no longer holds any reference to
// the removed value.
// Complexity is O(log n).
func (q *IndexPQ[T]) Pop() (k int, v T, found bool) {
	if q == nil || q.length == 0 {
		return
	}
	k, v = q.deletePos(0)
	return k, v, true
}

// Len returns the number of indices in the queue.
// Complexity is O(1).
func (q *IndexPQ[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// IsEmpty returns true if the queue has no indices in it.
// Complexity is O(1).
func (q *IndexPQ[T]) IsEmpty() bool {
	return q.Len() == 0
}

// Truncate removes all indices from the queue.  The comparison function
// and the index space 0..n-1 are kept, so the queue remains usable and
// can simply be refilled.
// Complexity is O(n).
func (q *IndexPQ[T]) Truncate() {
	if q == nil {
		return
	}
	q.length = 0
	q.pq = q.pq[:0]
	for i := range q.qp {
		q.qp[i] = -1
	}
	clear(q.vals) // zero the value slots so the GC can reclaim them
}

// Lock is a no-op provided for API compatibility with the thread-safe
// index_pq_ts twin.  This implementation is not safe for concurrent use.
func (q *IndexPQ[T]) Lock() {}

// Unlock is a no-op provided for API compatibility with the thread-safe
// index_pq_ts twin.  This implementation is not safe for concurrent use.
func (q *IndexPQ[T]) Unlock() {}

// clone returns a private copy of the queue sharing only the comparison
// function; draining the copy cannot affect the original.
func (q *IndexPQ[T]) clone() *IndexPQ[T] {
	return &IndexPQ[T]{
		n:      q.n,
		length: q.length,
		pq:     slices.Clone(q.pq),
		qp:     slices.Clone(q.qp),
		vals:   slices.Clone(q.vals),
		cmp:    q.cmp,
	}
}

// All returns an iterator that yields the (index, value) pairs of the
// queue in priority order, minimum value first.
//
// The iteration is non-destructive: it drains a private copy of the
// queue (a snapshot taken when All is called), so the queue is unchanged
// afterwards and indices inserted, changed or removed after the call do
// not affect the sequence.  Mutating the queue from inside the loop is
// safe.
// Complexity is O(n) to build the snapshot, then O(log n) per pair as it
// drains.
//
//	for k, v := range q.All() {
//		fmt.Println(k, v)
//	}
func (q *IndexPQ[T]) All() iter.Seq2[int, T] {
	if q == nil {
		return func(func(int, T) bool) {} // a nil queue iterates as an empty one
	}
	snapshot := q.clone()
	return func(yield func(int, T) bool) {
		for snapshot.length > 0 {
			k, v := snapshot.deletePos(0)
			if !yield(k, v) {
				return
			}
		}
	}
}
