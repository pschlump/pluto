/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package merge_sort implements mergesort (John von Neumann, 1945) as
// plain in-place slice-sort functions, following Sedgwick & Wayne,
// Algorithms, 4th ed. §2.2 (Merge.java / MergeBU.java / MergeX.java):
// the divide-and-conquer sort that is STABLE — elements that compare
// equal keep their relative order — the property neither quick_sort
// (3-way quicksort) nor heap_sort (heapsort) provides.
//
// There are three function families, all stateless:
//
//	Sort / SortFunc          top-down recursive mergesort (algs4
//	                         MergeX lineage: insertion-sort cutoff,
//	                         already-ordered merge skip)
//	SortBottomUp / SortBottomUpFunc   bottom-up iterative variant
//	                         (algs4 MergeBU — no recursion at all)
//	Inversions / InversionsFunc   counts the inversions of a slice via
//	                         the merge — the classic mergesort rider
//
// Sort orders naturally ordered element types (all integers, floats
// and strings — cmp.Ordered) by the built-in < and > operators;
// SortFunc orders any type by a caller supplied comparison function.
// The element type never implements an interface — no boxing, no
// dynamic dispatch, no type assertions.
//
// Stability comes from the merge rule: when the two front elements
// compare equal, the one from the LEFT half is taken first.  The
// insertion-sort cutoff uses the same strictly-greater shift, so the
// small-array path is stable too.
//
// Mergesort needs an auxiliary slice — the sort is in-place in the
// sense that the input is rearranged into order, but it allocates
// O(n) workspace (allocated once per call, not per merge).  Nil,
// empty and single-element slices are no-ops.  The package panics in
// exactly one situation, a programmer error: a nil comparison
// function passed to any *Func form.
//
// The functions hold no package state and are safe for concurrent use
// on disjoint slices, so no _ts twin is needed (the quick_sort
// precedent).
package merge_sort

import (
	"cmp"
	"slices"
)

// insertionCutoff is the subarray size below which the top-down sort
// switches to a straight insertion sort (algs4 MergeX CUTOFF = 7).
const insertionCutoff = 7

// Sort sorts data in place in ascending order, using the built-in < and
// > operators of T.  The sort is STABLE: elements that compare equal
// keep their relative order.  Nil, empty and single-element slices are
// no-ops.
// Complexity is O(n log₂ n) compares in every case (best, average and
// worst — no adversarial input), with O(n) auxiliary space.
func Sort[T cmp.Ordered](data []T) {
	sortTopDown(data, make([]T, len(data)), 0, len(data)-1, cmp.Compare[T])
}

// SortFunc sorts data in place in ascending order, using the caller
// supplied comparison function cmpFn (negative if a < b, 0 if equal,
// positive if a > b) — for example a struct ordered by one of its
// fields.  A reversed comparison function yields descending order.
// The sort is STABLE: elements that compare equal keep their relative
// order.  Nil, empty and single-element slices are no-ops.
// It panics if cmpFn is nil.
// Complexity is O(n log₂ n) compares in every case, with O(n)
// auxiliary space.
func SortFunc[T any](data []T, cmpFn func(a, b T) int) {
	if cmpFn == nil {
		panic("merge_sort: SortFunc called with a nil comparison function")
	}
	sortTopDown(data, make([]T, len(data)), 0, len(data)-1, cmpFn)
}

// SortBottomUp sorts data in place in ascending order, using the
// built-in < and > operators of T, by the bottom-up (iterative)
// variant: passes of width 1, 2, 4, 8, ... merge adjacent sorted runs
// until the whole slice is one run.  No recursion at all — a fixed
// pair of nested loops (the algs4 MergeBU lineage, kept plain: no
// insertion cutoff, no already-ordered skip).  The sort is STABLE,
// like every sort in this package.
// Complexity is O(n log₂ n) compares in every case, with O(n)
// auxiliary space.
func SortBottomUp[T cmp.Ordered](data []T) {
	sortBottomUp(data, make([]T, len(data)), cmp.Compare[T])
}

// SortBottomUpFunc is SortBottomUp with a caller supplied comparison
// function (negative if a < b, 0 if equal, positive if a > b).  The
// sort is STABLE.  Nil, empty and single-element slices are no-ops.
// It panics if cmpFn is nil.
// Complexity is O(n log₂ n) compares in every case, with O(n)
// auxiliary space.
func SortBottomUpFunc[T any](data []T, cmpFn func(a, b T) int) {
	if cmpFn == nil {
		panic("merge_sort: SortBottomUpFunc called with a nil comparison function")
	}
	sortBottomUp(data, make([]T, len(data)), cmpFn)
}

// Inversions returns the number of inversions in data — pairs
// i < j with data[i] > data[j].  A sorted slice has 0, a
// reverse-sorted slice has n·(n−1)/2 (the maximum), and every swap of
// adjacent elements changes the count by exactly 1, so the value is
// the minimum number of adjacent swaps that would sort the slice.
// The count is computed by the merge of a mergesort, O(n log₂ n); the
// input is NOT modified (a private copy is sorted).
func Inversions[T cmp.Ordered](data []T) int64 {
	return inversionsFunc(data, cmp.Compare[T])
}

// InversionsFunc is Inversions with a caller supplied comparison
// function (negative if a < b, 0 if equal, positive if a > b).
// The input is NOT modified.
// It panics if cmpFn is nil.
// Complexity is O(n log₂ n) compares with O(n) auxiliary space.
func InversionsFunc[T any](data []T, cmpFn func(a, b T) int) int64 {
	if cmpFn == nil {
		panic("merge_sort: InversionsFunc called with a nil comparison function")
	}
	return inversionsFunc(data, cmpFn)
}

// inversionsFunc is the shared Inversions body: sort a private copy
// top-down, counting the inversions the merge repairs.
func inversionsFunc[T any](data []T, cmpFn func(a, b T) int) int64 {
	a := slices.Clone(data)
	aux := make([]T, len(a))
	return countInversions(a, aux, 0, len(a)-1, cmpFn)
}

// sortTopDown is the algs4 MergeX-style recursive mergesort over
// data[lo..hi]: insertion-sort small subarrays, recurse on both
// halves, skip the merge when the halves are already in order.
func sortTopDown[T any](data, aux []T, lo, hi int, cmpFn func(a, b T) int) {
	if hi-lo < insertionCutoff {
		insertionSort(data, lo, hi, cmpFn)
		return
	}
	mid := lo + (hi-lo)/2
	sortTopDown(data, aux, lo, mid, cmpFn)
	sortTopDown(data, aux, mid+1, hi, cmpFn)
	// Already ordered (the last of the left half <= first of the
	// right)?  The merge would move nothing — skip it.
	if cmpFn(data[mid], data[mid+1]) <= 0 {
		return
	}
	mergeInto(data, aux, lo, mid, hi, cmpFn)
}

// sortBottomUp is the algs4 MergeBU iterative mergesort: merge runs
// of width 1, then 2, then 4, ... until one run covers the slice.
func sortBottomUp[T any](data, aux []T, cmpFn func(a, b T) int) {
	n := len(data)
	for width := 1; width < n; width *= 2 {
		for lo := 0; lo < n-width; lo += 2 * width {
			mid := lo + width - 1
			hi := min(lo+2*width-1, n-1)
			mergeInto(data, aux, lo, mid, hi, cmpFn)
		}
	}
}

// mergeInto merges the sorted runs data[lo..mid] and
// data[mid+1..hi] in place, using aux as the workspace (the whole
// range is copied there, then merged back).  The merge is STABLE: on
// a tie the element from the left run is taken first.
func mergeInto[T any](data, aux []T, lo, mid, hi int, cmpFn func(a, b T) int) {
	copy(aux[lo:hi+1], data[lo:hi+1])

	i, j := lo, mid+1
	for k := lo; k <= hi; k++ {
		if i > mid {
			data[k] = aux[j]
			j++
		} else if j > hi {
			data[k] = aux[i]
			i++
		} else if cmpFn(aux[j], aux[i]) < 0 {
			data[k] = aux[j] // strictly smaller — the right run wins,
			j++              // ties take the left run (stability)
		} else {
			data[k] = aux[i]
			i++
		}
	}
}

// countInversions sorts data[lo..hi] by top-down mergesort while
// counting the inversions the merge repairs: when an element is taken
// from the right run, every remaining element of the left run is an
// inversion with it.
func countInversions[T any](data, aux []T, lo, hi int, cmpFn func(a, b T) int) int64 {
	if hi <= lo {
		return 0
	}
	mid := lo + (hi-lo)/2
	inv := countInversions(data, aux, lo, mid, cmpFn)
	inv += countInversions(data, aux, mid+1, hi, cmpFn)

	copy(aux[lo:hi+1], data[lo:hi+1])
	i, j := lo, mid+1
	for k := lo; k <= hi; k++ {
		if i > mid {
			data[k] = aux[j]
			j++
		} else if j > hi {
			data[k] = aux[i]
			i++
		} else if cmpFn(aux[j], aux[i]) < 0 {
			data[k] = aux[j]
			j++
			inv += int64(mid - i + 1) // aux[i..mid] all invert with aux[j]
		} else {
			data[k] = aux[i]
			i++
		}
	}
	return inv
}

// insertionSort sorts data[lo..hi] (inclusive) by the stable
// hoisted-value form: an element shifts past only STRICTLY greater
// predecessors, so equals keep their relative order — the same form
// quick_sort's cutoff path uses.
func insertionSort[T any](data []T, lo, hi int, cmpFn func(a, b T) int) {
	for i := lo + 1; i <= hi; i++ {
		v := data[i]
		j := i
		for j > lo && cmpFn(data[j-1], v) > 0 {
			data[j] = data[j-1]
			j--
		}
		data[j] = v
	}
}
