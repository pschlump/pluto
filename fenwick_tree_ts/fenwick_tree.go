/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package fenwick_tree_ts implements a Fenwick tree (binary indexed
// tree) over the indices 0..n-1, safe for concurrent use.  It is the
// thread-safe twin of github.com/pschlump/pluto/fenwick_tree — the same
// API, guarded by a sync.RWMutex — with the addition of the Lock/Unlock
// pair and the Nl-prefixed (no-lock) methods for compound operations.
//
// Basic operations:
//
//	Add — Adds delta to the value at i.						O(log n)
//	Set — Assigns v to the value at i.						O(log n)
//	Sum — Returns the sum of the values at 0..i inclusive.	O(log n)
//	RangeSum — Returns the sum over [lo, hi] inclusive.		O(log n)
//	Value — Returns the value at i.							O(log n)
//	Len — Returns n, the number of slots.					O(1)
//
// Out-of-range indices REPORT, they do not panic: Add and Set return
// false, Value and RangeSum report ok=false, and Sum returns the zero
// value of T (Sum(-1) is the empty prefix).
//
// The package has exactly three panics, identical to fenwick_tree:
//
//	NewFenwickTree(n) with n < 1.
//	NewFenwickTreeFrom(data) with an empty slice.
//	Add or Set on a nil *FenwickTree.
//
// A nil *FenwickTree and the zero value behave as an empty Fenwick
// tree for every other operation.
//
// Concurrency model: Add and Set take the WRITE lock; Sum, RangeSum,
// Value and Len are true readers and take the read lock.  (Unlike
// union_find_ts, no read here mutates the structure — prefix sums and
// value reconstruction are pure.)  Nil guards come BEFORE the lock
// acquisition.  For compound operations (e.g. read-then-update) take
// the real Lock and use the Nl-prefixed methods; calling a regular
// method while holding Lock deadlocks.
//
// Run the tests with -race.
package fenwick_tree_ts

import (
	"sync"

	"github.com/pschlump/pluto/g_lib"
)

// FenwickTree is a Fenwick (binary indexed) tree over the slots
// 0..n-1, safe for concurrent use.  See the package documentation for
// the locking model.
//
// tree is the 1-based internal array: tree[0] is unused, and tree[k]
// holds the sum of the lowbit(k) values ending at slot k-1.
type FenwickTree[T g_lib.Numeric] struct {
	tree []T
	lock sync.RWMutex
}

// NewFenwickTree returns a Fenwick tree over the slots 0..n-1, with
// every value zero.  It panics if n < 1.
// Complexity is O(n).
func NewFenwickTree[T g_lib.Numeric](n int) *FenwickTree[T] {
	if n < 1 {
		panic("fenwick_tree_ts: NewFenwickTree called with n < 1")
	}
	return &FenwickTree[T]{tree: make([]T, n+1)}
}

// NewFenwickTreeFrom returns a Fenwick tree over the slots
// 0..len(data)-1 initialized with the given values: Value(i) == data[i].
// It builds in O(n) by summing each slot into its parent range rather
// than by n calls to Add.  It panics if data is empty.
// Complexity is O(n).
func NewFenwickTreeFrom[T g_lib.Numeric](data []T) *FenwickTree[T] {
	if len(data) == 0 {
		panic("fenwick_tree_ts: NewFenwickTreeFrom called with an empty slice")
	}
	n := len(data)
	ft := &FenwickTree[T]{tree: make([]T, n+1)}
	copy(ft.tree[1:], data)
	for i := 1; i <= n; i++ {
		if j := i + (i & -i); j <= n {
			ft.tree[j] += ft.tree[i]
		}
	}
	return ft
}

// inRange reports whether i is a valid slot index of ft.  The caller
// must hold the lock (or ft must be unshared).
func (ft *FenwickTree[T]) inRange(i int) bool {
	return i >= 0 && i < len(ft.tree)-1
}

// -------------------------------------------------------------------------------------------------------
// Lock-free internals; the caller must hold the appropriate lock.
// -------------------------------------------------------------------------------------------------------

// nlAdd is Add without locking or the range check; the caller must
// hold the write lock and must have verified that i is in range.
func (ft *FenwickTree[T]) nlAdd(i int, delta T) {
	for k := i + 1; k < len(ft.tree); k += k & -k {
		ft.tree[k] += delta
	}
}

// nlSum is Sum without locking or the range check; the caller must
// hold the read lock and must have verified that i is in range.
func (ft *FenwickTree[T]) nlSum(i int) (rv T) {
	for k := i + 1; k > 0; k -= k & -k {
		rv += ft.tree[k]
	}
	return
}

// nlValue is Value without locking or the range check; the caller must
// hold the read lock and must have verified that i is in range.
func (ft *FenwickTree[T]) nlValue(i int) T {
	return ft.nlSum(i) - ft.nlSum(i-1)
}

// -------------------------------------------------------------------------------------------------------
// Public API
// -------------------------------------------------------------------------------------------------------

// Add adds delta to the value at slot i and returns true.  It returns
// false — changing nothing — if i is out of range.
//
// It panics on a nil *FenwickTree: a nil structure cannot record an
// update (one of the package's two method panics).
// Complexity is O(log n).
func (ft *FenwickTree[T]) Add(i int, delta T) bool {
	if ft == nil {
		panic("fenwick_tree_ts: Add called on a nil FenwickTree")
	}
	ft.lock.Lock()
	defer ft.lock.Unlock()
	if !ft.inRange(i) {
		return false
	}
	ft.nlAdd(i, delta)
	return true
}

// Set assigns v to the value at slot i and returns true.  It returns
// false — changing nothing — if i is out of range.  Set computes the
// delta from the current value and applies it with the lock-free Add
// body, holding the write lock across the read and the update.
//
// It panics on a nil *FenwickTree (one of the package's two method
// panics).
// Complexity is O(log n).
func (ft *FenwickTree[T]) Set(i int, v T) bool {
	if ft == nil {
		panic("fenwick_tree_ts: Set called on a nil FenwickTree")
	}
	ft.lock.Lock()
	defer ft.lock.Unlock()
	if !ft.inRange(i) {
		return false
	}
	ft.nlAdd(i, v-ft.nlValue(i))
	return true
}

// Sum returns the sum of the values at slots 0..i inclusive.  It
// returns the zero value of T — the empty sum — if i is out of range
// (including i == -1, which is the empty prefix) or ft is nil or a
// zero value.
// Complexity is O(log n).
func (ft *FenwickTree[T]) Sum(i int) (rv T) {
	if ft == nil {
		return
	}
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	if !ft.inRange(i) {
		return
	}
	return ft.nlSum(i)
}

// RangeSum returns the sum of the values at slots lo..hi inclusive.
// ok is false — and the sum is the zero value of T — if the range is
// invalid (lo < 0, hi >= Len(), or lo > hi) or ft is nil or a zero
// value.
// Complexity is O(log n).
func (ft *FenwickTree[T]) RangeSum(lo, hi int) (rv T, ok bool) {
	if ft == nil {
		return rv, false
	}
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	if !ft.inRange(lo) || !ft.inRange(hi) || lo > hi {
		return rv, false
	}
	return ft.nlSum(hi) - ft.nlSum(lo-1), true
}

// Value returns the value at slot i.  ok is false — and the value is
// the zero value of T — if i is out of range or ft is nil or a zero
// value.  The value is reconstructed as the difference of two prefix
// sums.
// Complexity is O(log n).
func (ft *FenwickTree[T]) Value(i int) (rv T, ok bool) {
	if ft == nil {
		return rv, false
	}
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	if !ft.inRange(i) {
		return rv, false
	}
	return ft.nlValue(i), true
}

// Len returns n, the number of slots (0..n-1).  A nil or zero-value
// Fenwick tree reports 0.
// Complexity is O(1).
func (ft *FenwickTree[T]) Len() int {
	if ft == nil {
		return 0
	}
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	return ft.NlLen()
}

// IsEmpty returns true if the Fenwick tree has no slots.  Only a nil
// or zero-value tree is empty — both constructors require n ≥ 1.
// Complexity is O(1).
func (ft *FenwickTree[T]) IsEmpty() bool {
	return ft.Len() == 0
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
func (ft *FenwickTree[T]) Lock() {
	if ft == nil {
		return
	}
	ft.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  Unlocking a nil tree
// is a no-op.
func (ft *FenwickTree[T]) Unlock() {
	if ft == nil {
		return
	}
	ft.lock.Unlock()
}

// NlLen is Len without locking; call it only while holding Lock.
func (ft *FenwickTree[T]) NlLen() int {
	if len(ft.tree) == 0 {
		return 0
	}
	return len(ft.tree) - 1
}

// NlSum is Sum without locking; call it only while holding Lock.  An
// out-of-range i (including -1) yields the zero value of T.
func (ft *FenwickTree[T]) NlSum(i int) (rv T) {
	if !ft.inRange(i) {
		return
	}
	return ft.nlSum(i)
}

// NlRangeSum is RangeSum without locking; call it only while holding
// Lock.  It reports false on an invalid range.
func (ft *FenwickTree[T]) NlRangeSum(lo, hi int) (rv T, ok bool) {
	if !ft.inRange(lo) || !ft.inRange(hi) || lo > hi {
		return rv, false
	}
	return ft.nlSum(hi) - ft.nlSum(lo-1), true
}

// NlValue is Value without locking; call it only while holding Lock.
// It reports false if i is out of range.
func (ft *FenwickTree[T]) NlValue(i int) (rv T, ok bool) {
	if !ft.inRange(i) {
		return rv, false
	}
	return ft.nlValue(i), true
}

// NlAdd is Add without locking; call it only while holding Lock.  It
// reports false if i is out of range.
func (ft *FenwickTree[T]) NlAdd(i int, delta T) bool {
	if !ft.inRange(i) {
		return false
	}
	ft.nlAdd(i, delta)
	return true
}

// NlSet is Set without locking; call it only while holding Lock.  It
// reports false if i is out of range.
func (ft *FenwickTree[T]) NlSet(i int, v T) bool {
	if !ft.inRange(i) {
		return false
	}
	ft.nlAdd(i, v-ft.nlValue(i))
	return true
}
