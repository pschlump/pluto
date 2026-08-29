/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package fenwick_tree implements a Fenwick tree (binary indexed tree)
// over the indices 0..n-1 — the classic prefix-sum structure of the
// "Beyond" section of Sedgwick & Wayne's algs4 code list.
//
// A Fenwick tree maintains an array of n numeric values and answers
// prefix-sum queries in O(log n) with point updates in O(log n), using
// exactly n slots of storage (no node objects, no recursion).  Range
// sums are the difference of two prefix sums, and a point value is the
// difference of two adjacent prefix sums.
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
// Out-of-range indices REPORT, they do not panic (the heap
// indexed-operation convention): Add and Set return false, Value and
// RangeSum report ok=false, and Sum returns the zero value of T (a
// Sum on any out-of-range index, including Sum(-1), is the empty sum).
//
// The package has exactly three panics:
//
//	NewFenwickTree(n) with n < 1 — a Fenwick tree over no slots is
//	meaningless.
//	NewFenwickTreeFrom(data) with an empty slice — same reason.
//	Add or Set on a nil *FenwickTree — the writes with no sane answer.
//
// A nil *FenwickTree and the zero value behave as an empty Fenwick
// tree (no slots) for every other operation.
//
// The structure is not safe for concurrent use — see the thread-safe
// twin fenwick_tree_ts.
package fenwick_tree

import (
	"github.com/pschlump/pluto/g_lib"
)

// FenwickTree is a Fenwick (binary indexed) tree over the slots
// 0..n-1, holding values of any numeric type.
//
// tree is the 1-based internal array: tree[0] is unused, and tree[k]
// holds the sum of the lowbit(k) values ending at slot k-1.  The slot
// count is len(tree)-1.
type FenwickTree[T g_lib.Numeric] struct {
	tree []T
}

// NewFenwickTree returns a Fenwick tree over the slots 0..n-1, with
// every value zero.  It panics if n < 1.
// Complexity is O(n).
func NewFenwickTree[T g_lib.Numeric](n int) *FenwickTree[T] {
	if n < 1 {
		panic("fenwick_tree: NewFenwickTree called with n < 1")
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
		panic("fenwick_tree: NewFenwickTreeFrom called with an empty slice")
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

// inRange reports whether i is a valid slot index of ft.
func (ft *FenwickTree[T]) inRange(i int) bool {
	return ft != nil && i >= 0 && i < len(ft.tree)-1
}

// Add adds delta to the value at slot i and returns true.  It returns
// false — changing nothing — if i is out of range.
//
// It panics on a nil *FenwickTree: a nil structure cannot record an
// update (one of the package's two method panics).
// Complexity is O(log n).
func (ft *FenwickTree[T]) Add(i int, delta T) bool {
	if ft == nil {
		panic("fenwick_tree: Add called on a nil FenwickTree")
	}
	if !ft.inRange(i) {
		return false
	}
	for k := i + 1; k < len(ft.tree); k += k & -k {
		ft.tree[k] += delta
	}
	return true
}

// Set assigns v to the value at slot i and returns true.  It returns
// false — changing nothing — if i is out of range.  Set computes the
// delta from the current value and applies it with Add.
//
// It panics on a nil *FenwickTree (one of the package's two method
// panics).
// Complexity is O(log n).
func (ft *FenwickTree[T]) Set(i int, v T) bool {
	if ft == nil {
		panic("fenwick_tree: Set called on a nil FenwickTree")
	}
	if !ft.inRange(i) {
		return false
	}
	return ft.Add(i, v-ft.nlValue(i))
}

// Sum returns the sum of the values at slots 0..i inclusive.  It
// returns the zero value of T — the empty sum — if i is out of range
// (including i == -1, which is the empty prefix) or ft is nil or a
// zero value.
// Complexity is O(log n).
func (ft *FenwickTree[T]) Sum(i int) (rv T) {
	if ft == nil || i < 0 || i >= len(ft.tree)-1 {
		return
	}
	for k := i + 1; k > 0; k -= k & -k {
		rv += ft.tree[k]
	}
	return
}

// RangeSum returns the sum of the values at slots lo..hi inclusive.
// ok is false — and the sum is the zero value of T — if the range is
// invalid (lo < 0, hi >= Len(), or lo > hi) or ft is nil or a zero
// value.
// Complexity is O(log n).
func (ft *FenwickTree[T]) RangeSum(lo, hi int) (rv T, ok bool) {
	if !ft.inRange(lo) || !ft.inRange(hi) || lo > hi {
		return rv, false
	}
	return ft.Sum(hi) - ft.Sum(lo-1), true
}

// Value returns the value at slot i.  ok is false — and the value is
// the zero value of T — if i is out of range or ft is nil or a zero
// value.  The value is reconstructed as the difference of two prefix
// sums.
// Complexity is O(log n).
func (ft *FenwickTree[T]) Value(i int) (rv T, ok bool) {
	if !ft.inRange(i) {
		return rv, false
	}
	return ft.nlValue(i), true
}

// nlValue is Value without the range check; the caller must have
// verified that i is in range.
func (ft *FenwickTree[T]) nlValue(i int) T {
	return ft.Sum(i) - ft.Sum(i-1)
}

// Len returns n, the number of slots (0..n-1).  A nil or zero-value
// Fenwick tree reports 0.
// Complexity is O(1).
func (ft *FenwickTree[T]) Len() int {
	if ft == nil || len(ft.tree) == 0 {
		return 0
	}
	return len(ft.tree) - 1
}

// IsEmpty returns true if the Fenwick tree has no slots.  Only a nil
// or zero-value tree is empty — both constructors require n ≥ 1.
// Complexity is O(1).
func (ft *FenwickTree[T]) IsEmpty() bool {
	return ft.Len() == 0
}

// Lock is a no-op in this package; it exists for API parity with the
// thread-safe twin fenwick_tree_ts, where Lock takes the write lock.
func (ft *FenwickTree[T]) Lock() {}

// Unlock is a no-op in this package; see Lock.
func (ft *FenwickTree[T]) Unlock() {}
