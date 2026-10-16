/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package string_sort

import "fmt"

// LSDSort sorts fixed-width string keys in place with w passes of
// key-indexed counting, one pass per byte position from the least
// significant (byte w-1) to the most significant (byte 0) — algs4
// §5.1 LSD.java.  Each pass is a stable counting sort, and running the
// passes least-significant-first composes them into a sort by the
// first w bytes of every key.
//
// Every key must be at least w bytes long.  The textbook contract is
// keys of exactly w bytes; longer keys are legal but only their first
// w bytes are significant — ties on those w bytes keep their input
// order (the sort is stable).  Use MSDSort or Quick3String for
// variable-width keys.
//
// It panics if w is negative, or if any key is shorter than w — the
// only two panics in the package.  Nil, empty and single-element
// slices are no-ops.
// Complexity is O(w·(n+R)) time and O(n+R) extra space; the sort is
// stable.
func LSDSort(a []string, w int) {
	if w < 0 {
		panic(fmt.Sprintf("string_sort: LSDSort called with w = %d — w must be >= 0 (the number of key bytes to sort by)", w))
	}
	for i, s := range a {
		if len(s) < w {
			panic(fmt.Sprintf("string_sort: LSDSort: key at index %d has length %d, shorter than w = %d — LSD reads bytes 0..%d of every key; use MSDSort or Quick3String for variable-width keys", i, len(s), w, w-1))
		}
	}

	n := len(a)
	if n <= 1 {
		return
	}
	aux := make([]string, n)
	var count [R + 1]int
	for d := w - 1; d >= 0; d-- {
		clear(count[:])

		// Key-indexed counting on byte d: frequency counts, cumulates,
		// stable distribute into aux, copy back.
		for i := range a {
			count[int(a[i][d])+1]++
		}
		for r := 0; r < R; r++ {
			count[r+1] += count[r]
		}
		for i := range a {
			c := int(a[i][d])
			aux[count[c]] = a[i]
			count[c]++
		}
		copy(a, aux)
	}
}
