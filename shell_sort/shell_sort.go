/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package shell_sort implements shellsort (Donald L. Shell, 1959) as
// plain in-place slice-sort functions, following Sedgwick & Wayne,
// Algorithms, 4th ed. §2.1 (Shell.java): an h-sort by gapped insertion
// for a decreasing gap sequence 1, 4, 13, 40, 121, ... (each gap is
// 3h+1), finishing with h = 1 — a plain insertion sort over data that
// is already "almost sorted", which is why it beats quadratic.
//
// There is no sorter object, no constructor and no stored state.  Sort
// orders naturally ordered element types (all integers, floats and
// strings — cmp.Ordered) by the built-in < and > operators; SortFunc
// orders any type by a caller supplied comparison function.  The
// element type never implements an interface — no boxing, no dynamic
// dispatch, no type assertions.
//
// The sort is NOT stable: elements that compare equal may come out in
// any relative order.
//
// The 3x+1 gap sequence gives a worst case of O(n^{3/2}) compares; the
// exact average is a long-standing open problem (empirically about
// O(n^{1.25}) on random input — Sedgwick's own measurement).  Unlike
// quick_sort there is no recursion at all: the sort is a fixed pair of
// nested loops, so there is no stack-depth story and no adversarial
// input shape — the gap schedule is the same for every input.
//
// Nil, empty and single-element slices are no-ops.  The package panics
// in exactly one situation, a programmer error:
//
//	SortFunc(data, nil)  — nil comparison function.
//
// Both functions are stateless and safe for concurrent use on disjoint
// slices; there is no shared package state, so no _ts twin is needed.
package shell_sort

import (
	"cmp"
)

// Sort sorts data in place in ascending order, using the built-in < and
// > operators of T.  Nil, empty and single-element slices are no-ops.
// The sort is NOT stable: elements that compare equal may come out in
// any relative order.
// Complexity is O(n^{3/2}) in the worst case with the 3x+1 gap sequence
// (1, 4, 13, 40, ...), about O(n^{1.25}) empirically on random input,
// O(n log n) compares on already-sorted input (each of the ~log₃ n
// passes is a linear scan that moves nothing).
func Sort[T cmp.Ordered](data []T) {
	shell(data, cmp.Compare[T])
}

// SortFunc sorts data in place in ascending order, using the caller
// supplied comparison function cmpFn (negative if a < b, 0 if equal,
// positive if a > b) — for example a struct ordered by one of its
// fields.  A reversed comparison function yields descending order.
// Nil, empty and single-element slices are no-ops.  The sort is NOT
// stable: elements that compare equal may come out in any relative
// order.
// It panics if cmpFn is nil — the only panic in the package.
// Complexity is O(n^{3/2}) in the worst case with the 3x+1 gap sequence,
// about O(n^{1.25}) empirically on random input, O(n log n) compares on
// already-sorted input.
func SortFunc[T any](data []T, cmpFn func(a, b T) int) {
	if cmpFn == nil {
		panic("shell_sort: SortFunc called with a nil comparison function")
	}
	shell(data, cmpFn)
}

// shell h-sorts data for each gap h in the decreasing 3x+1 sequence
// (1, 4, 13, 40, 121, ... down to 1), exactly the algs4 Shell.sort gap
// schedule.  Each pass is a gapped insertion sort written in the
// hoisted-value form (the algs4 code uses exch; the hoisted form does
// the same compares with fewer writes — it is the same form
// quick_sort's insertion sort uses).
func shell[T any](data []T, cmpFn func(a, b T) int) {
	n := len(data)
	h := 1
	for h < n/3 {
		h = 3*h + 1 // 1, 4, 13, 40, 121, 364, 1093, ...
	}
	for h >= 1 {
		for i := h; i < n; i++ {
			v := data[i]
			j := i
			for j >= h && cmpFn(data[j-h], v) > 0 {
				data[j] = data[j-h]
				j -= h
			}
			data[j] = v
		}
		h /= 3
	}
}
