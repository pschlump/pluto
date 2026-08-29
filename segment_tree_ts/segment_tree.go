/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package segment_tree_ts implements an array-backed segment tree over
// the indices 0..n-1, safe for concurrent use.  It is the thread-safe
// twin of github.com/pschlump/pluto/segment_tree — the same API,
// guarded by a sync.RWMutex — with the addition of the Lock/Unlock
// pair and the Nl-prefixed (no-lock) methods for compound operations.
//
// Following the pluto ordering convention there are two constructors:
//
//	NewSegmentTree[T g_lib.Numeric](data) — range SUM over the type's
//	built-in + operator (identity 0).
//	NewSegmentTreeFunc[T any](data, combine, identity) — an arbitrary
//	associative combine function with its identity element.
//
// Basic operations:
//
//	Query — Returns the combine of the values over [lo, hi].	O(log n)
//	Update — Assigns v to the value at i.						O(log n)
//	Value — Returns the value at i.								O(1)
//	Len — Returns n, the number of slots.						O(1)
//
// Out-of-range indices REPORT, they do not panic: Update returns
// false, Query and Value report ok=false.
//
// The package has exactly four panics, identical to segment_tree:
//
//	NewSegmentTree(data) or NewSegmentTreeFunc(data, ...) with an
//	empty slice.
//	NewSegmentTreeFunc with a nil combine function.
//	Update on a nil *SegmentTree.
//
// A nil *SegmentTree and the zero value behave as an empty segment
// tree for every other operation.
//
// Concurrency model: Update takes the WRITE lock; Query, Value and
// Len are true readers and take the read lock.  Nil guards come
// BEFORE the lock acquisition.  For compound operations (e.g.
// read-then-update) take the real Lock and use the Nl-prefixed
// methods; calling a regular method while holding Lock deadlocks.
//
// Run the tests with -race.
package segment_tree_ts

import (
	"sync"

	"github.com/pschlump/pluto/g_lib"
)

// SegmentTree is an array-backed segment tree over the slots 0..n-1,
// safe for concurrent use.  See the package documentation for the
// locking model.
//
// The tree is stored in a slice of size 2*size, where size is the
// smallest power of two ≥ n: slot i's value lives at tree[size+i],
// and every internal node tree[k] holds combine(tree[2k], tree[2k+1]).
// Padding leaves (beyond size+n) hold the identity element.
type SegmentTree[T any] struct {
	tree     []T
	n        int // number of slots (the data length), 0 for a zero value
	size     int // leaves capacity: the smallest power of two ≥ n
	combine  func(a, b T) T
	identity T
	lock     sync.RWMutex
}

// NewSegmentTree returns a segment tree over the slots
// 0..len(data)-1 initialized with the given values, combining by the
// type's built-in + operator (a range-SUM tree, identity 0).
// It panics if data is empty.
// Complexity is O(n).
func NewSegmentTree[T g_lib.Numeric](data []T) *SegmentTree[T] {
	if len(data) == 0 {
		panic("segment_tree_ts: NewSegmentTree called with an empty slice")
	}
	return NewSegmentTreeFunc(data, func(a, b T) T { return a + b }, T(0))
}

// NewSegmentTreeFunc returns a segment tree over the slots
// 0..len(data)-1 initialized with the given values, combining ranges
// with the caller-supplied function.  combine must be associative and
// must have identity as its identity element (combine(identity, x) ==
// combine(x, identity) == x).  It panics if combine is nil or data is
// empty.
// Complexity is O(n).
func NewSegmentTreeFunc[T any](data []T, combine func(a, b T) T, identity T) *SegmentTree[T] {
	if combine == nil {
		panic("segment_tree_ts: NewSegmentTreeFunc called with a nil combine function")
	}
	if len(data) == 0 {
		panic("segment_tree_ts: NewSegmentTreeFunc called with an empty slice")
	}
	n := len(data)
	size := 1
	for size < n {
		size *= 2
	}
	st := &SegmentTree[T]{
		tree:     make([]T, 2*size),
		n:        n,
		size:     size,
		combine:  combine,
		identity: identity,
	}
	for i := range st.tree {
		st.tree[i] = identity
	}
	copy(st.tree[size:], data)
	for k := size - 1; k >= 1; k-- {
		st.tree[k] = combine(st.tree[2*k], st.tree[2*k+1])
	}
	return st
}

// inRange reports whether i is a valid slot index of st.  The caller
// must hold the lock (or st must be unshared).
func (st *SegmentTree[T]) inRange(i int) bool {
	return i >= 0 && i < st.n
}

// -------------------------------------------------------------------------------------------------------
// Lock-free internals; the caller must hold the appropriate lock.
// -------------------------------------------------------------------------------------------------------

// nlQuery is Query without locking or the range check; the caller must
// hold the read lock and must have verified the range.
func (st *SegmentTree[T]) nlQuery(lo, hi int) T {
	// Iterative query on the leaf row [lo+size, hi+size]: accumulate
	// the left fringe in order and the right fringe in reverse order.
	left := st.identity
	right := st.identity
	for l, r := lo+st.size, hi+st.size; l <= r; l, r = l/2, r/2 {
		if l%2 == 1 { // l is a right child: take it, step right
			left = st.combine(left, st.tree[l])
			l++
		}
		if r%2 == 0 { // r is a left child: take it, step left
			right = st.combine(st.tree[r], right)
			r--
		}
	}
	return st.combine(left, right)
}

// nlUpdate is Update without locking or the range check; the caller
// must hold the write lock and must have verified that i is in range.
func (st *SegmentTree[T]) nlUpdate(i int, v T) {
	k := i + st.size
	st.tree[k] = v
	for k /= 2; k >= 1; k /= 2 {
		st.tree[k] = st.combine(st.tree[2*k], st.tree[2*k+1])
	}
}

// -------------------------------------------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------------------------------------------

// Query returns the combine of the values at slots lo..hi inclusive.
// ok is false — and the result is the zero value of T — if the range
// is invalid (lo < 0, hi >= Len(), or lo > hi) or st is nil or a zero
// value.
// Complexity is O(log n).
func (st *SegmentTree[T]) Query(lo, hi int) (rv T, ok bool) {
	if st == nil {
		return rv, false
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	if !st.inRange(lo) || !st.inRange(hi) || lo > hi {
		return rv, false
	}
	return st.nlQuery(lo, hi), true
}

// Update assigns v to the value at slot i and returns true.  It
// returns false — changing nothing — if i is out of range.
//
// It panics on a nil *SegmentTree: a nil structure cannot record an
// update (the package's only method panic).
// Complexity is O(log n).
func (st *SegmentTree[T]) Update(i int, v T) bool {
	if st == nil {
		panic("segment_tree_ts: Update called on a nil SegmentTree")
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	if !st.inRange(i) {
		return false
	}
	st.nlUpdate(i, v)
	return true
}

// Value returns the value at slot i.  ok is false — and the value is
// the zero value of T — if i is out of range or st is nil or a zero
// value.
// Complexity is O(1).
func (st *SegmentTree[T]) Value(i int) (rv T, ok bool) {
	if st == nil {
		return rv, false
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	if !st.inRange(i) {
		return rv, false
	}
	return st.tree[st.size+i], true
}

// Len returns n, the number of slots (0..n-1).  A nil or zero-value
// segment tree reports 0.
// Complexity is O(1).
func (st *SegmentTree[T]) Len() int {
	if st == nil {
		return 0
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.NlLen()
}

// IsEmpty returns true if the segment tree has no slots.  Only a nil
// or zero-value tree is empty — both constructors require n ≥ 1.
// Complexity is O(1).
func (st *SegmentTree[T]) IsEmpty() bool {
	return st.Len() == 0
}

// -------------------------------------------------------------------------------------------------------
// Exposed write lock and the Nl (no-lock) methods for compound
// operations.  While the lock is held only call the Nl-prefixed methods;
// calling a regular method will deadlock.
// -------------------------------------------------------------------------------------------------------

// Lock takes the tree's write lock, allowing a group of operations to
// be performed atomically.  While the lock is held only call the
// Nl-prefixed (no-lock) methods; calling a regular method will
// deadlock.  Pair every Lock with a corresponding Unlock.  Locking a
// nil tree is a no-op.
func (st *SegmentTree[T]) Lock() {
	if st == nil {
		return
	}
	st.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil tree
// is a no-op.
func (st *SegmentTree[T]) Unlock() {
	if st == nil {
		return
	}
	st.lock.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (st *SegmentTree[T]) NlLen() int {
	return st.n
}

// NlQuery is Query without locking; call it only while holding Lock.
// It reports false on an invalid range.
func (st *SegmentTree[T]) NlQuery(lo, hi int) (rv T, ok bool) {
	if !st.inRange(lo) || !st.inRange(hi) || lo > hi {
		return rv, false
	}
	return st.nlQuery(lo, hi), true
}

// NlValue is Value without locking; call it only while holding Lock.
// It reports false if i is out of range.
func (st *SegmentTree[T]) NlValue(i int) (rv T, ok bool) {
	if !st.inRange(i) {
		return rv, false
	}
	return st.tree[st.size+i], true
}

// NlUpdate is Update without locking; call it only while holding
// Lock.  It reports false if i is out of range.
func (st *SegmentTree[T]) NlUpdate(i int, v T) bool {
	if !st.inRange(i) {
		return false
	}
	st.nlUpdate(i, v)
	return true
}
