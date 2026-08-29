/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package quick_sort implements quicksort with 3-way partitioning
// (Dijkstra's Dutch-national-flag partitioning — Sedgwick's algs4 §2.3
// Quick3way) as plain in-place slice-sort functions: there is no sorter
// object, no constructor and no stored state.  Sort orders naturally
// ordered element types (all integers, floats and strings — cmp.Ordered)
// by the built-in < and > operators; SortFunc orders any type by a
// caller supplied comparison function.  The element type never
// implements an interface — no boxing, no dynamic dispatch, no type
// assertions.
//
// Three-way partitioning splits the slice into < pivot, == pivot and
// > pivot in one pass, so a slice of all-equal keys is sorted in O(n)
// and inputs with many duplicate keys are handled especially well.
// The sort is NOT stable: elements that compare equal may come out in
// any relative order.
//
// A library cannot pre-shuffle its input the way the algs4 code does
// (that would put a random source in the API), so the algs4 shuffle
// that guarantees O(n log₂ n) behavior is replaced by three robustness
// measures: median-of-3 pivot selection, an insertion-sort cutoff for
// runs of 12 or fewer elements, and recursion into the SMALLER side
// partition with an iteration loop on the larger side — which bounds
// the recursion depth at O(log n) even on adversarial input (sorted,
// reverse-sorted, organ-pipe and sawtooth inputs are all handled
// without stack growth problems).
//
// Nil, empty and single-element slices are no-ops.  The package panics
// in exactly one situation, a programmer error:
//
//	SortFunc(data, nil)  — nil comparison function.
//
// Both functions are stateless and safe for concurrent use on disjoint
// slices; there is no shared package state, so no _ts twin is needed.
package quick_sort

import (
	"cmp"
)

// insertionSortCutoff is the partition size at or below which the
// sorter switches to insertion sort — small runs are cheaper to sort
// with a simple O(n²) pass than with recursive partitioning.
const insertionSortCutoff = 12

// Sort sorts data in place in ascending order, using the built-in < and
// > operators of T.  Nil, empty and single-element slices are no-ops.
// The sort is NOT stable: elements that compare equal may come out in
// any relative order.
// Complexity is O(n log₂ n) on average, O(n²) in the worst case (made
// astronomically unlikely by median-of-3 pivot selection combined with
// 3-way partitioning), O(n) on all-equal keys, with O(log n) stack.
func Sort[T cmp.Ordered](data []T) {
	sort3way(data, 0, len(data)-1, cmp.Compare[T])
}

// SortFunc sorts data in place in ascending order, using the caller
// supplied comparison function cmpFn (negative if a < b, 0 if equal,
// positive if a > b) — for example a struct ordered by one of its
// fields.  A reversed comparison function yields descending order.
// Nil, empty and single-element slices are no-ops.  The sort is NOT
// stable: elements that compare equal may come out in any relative
// order.
// It panics if cmpFn is nil — the only panic in the package.
// Complexity is O(n log₂ n) on average, O(n²) in the worst case (made
// astronomically unlikely by median-of-3 pivot selection combined with
// 3-way partitioning), O(n) on all-equal keys, with O(log n) stack.
func SortFunc[T any](data []T, cmpFn func(a, b T) int) {
	if cmpFn == nil {
		panic("quick_sort: SortFunc called with a nil comparison function")
	}
	sort3way(data, 0, len(data)-1, cmpFn)
}

// sort3way quicksorts the sub-slice data[lo..hi] using 3-way
// (Dutch-national-flag) partitioning around a median-of-3 pivot.  It
// recurses into the smaller side partition and loops on the larger
// side, so recursion depth is bounded by O(log n); runs of
// insertionSortCutoff or fewer elements are finished with insertion
// sort.
func sort3way[T any](data []T, lo, hi int, cmpFn func(a, b T) int) {
	for hi-lo >= insertionSortCutoff {
		// Median-of-3: order data[lo], data[mid], data[hi] so that the
		// median ends up at lo and becomes the pivot.  This replaces the
		// algs4 pre-shuffle — sorted and reverse-sorted inputs then pivot
		// on the true median instead of an extreme.
		mid := lo + (hi-lo)/2
		if cmpFn(data[mid], data[lo]) < 0 {
			data[lo], data[mid] = data[mid], data[lo]
		}
		if cmpFn(data[hi], data[lo]) < 0 {
			data[lo], data[hi] = data[hi], data[lo]
		}
		if cmpFn(data[hi], data[mid]) < 0 {
			data[mid], data[hi] = data[hi], data[mid]
		}
		data[lo], data[mid] = data[mid], data[lo]

		// Dijkstra Dutch-national-flag partition (algs4 Quick3way):
		// data[lo..lt-1] < v, data[lt..gt] == v, data[gt+1..hi] > v.
		v := data[lo]
		lt, i, gt := lo, lo+1, hi
		for i <= gt {
			switch c := cmpFn(data[i], v); {
			case c < 0:
				data[lt], data[i] = data[i], data[lt]
				lt++
				i++
			case c > 0:
				data[i], data[gt] = data[gt], data[i]
				gt--
			default:
				i++
			}
		}

		// Recurse into the smaller side and loop on the larger side.
		if lt-lo < hi-gt {
			sort3way(data, lo, lt-1, cmpFn)
			lo = gt + 1
		} else {
			sort3way(data, gt+1, hi, cmpFn)
			hi = lt - 1
		}
	}
	insertionSort(data, lo, hi, cmpFn)
}

// insertionSort sorts the sub-slice data[lo..hi] in place with a
// straight insertion sort.
func insertionSort[T any](data []T, lo, hi int, cmpFn func(a, b T) int) {
	for i := lo + 1; i <= hi; i++ {
		v := data[i]
		j := i - 1
		for j >= lo && cmpFn(data[j], v) > 0 {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = v
	}
}
