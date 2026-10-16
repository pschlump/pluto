/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package string_sort

// MSDSort sorts variable-width string keys in place,
// most-significant-byte-first — algs4 §5.1 MSD.java: key-indexed
// counting on the leading byte, then recursion inside each byte's
// bucket on the next byte.  A key that ends before the current
// position sorts first (the byteAt -1 sentinel), so a key that is a
// prefix of another sorts first, exactly as with Go's native string <.
//
// Subarrays of insertCutoff or fewer keys switch to an insertion sort
// on the remaining bytes (algs4's cutoff), which stops the recursion
// from degenerating one key at a time down a shared prefix.
//
// Nil, empty and single-element slices are no-ops.  There are no
// panics.
// Complexity is between O(n+R) and O(w·(n+R)) depending on how much
// the keys share prefixes (about n·log_R n bucket work on random
// keys); recursion depth is at most the length of the longest key; the
// sort is stable.  Extra space is one O(n) auxiliary slice shared by
// every recursion level plus O(R) counts per level.
func MSDSort(a []string) {
	n := len(a)
	if n <= 1 {
		return
	}
	aux := make([]string, n)
	msdSort(a, 0, n-1, 0, aux)
}

// msdSort sorts a[lo..hi] — keys known to share their first d bytes —
// on the bytes from position d on, using aux as shared scratch.  This
// is the body of algs4 MSD.sort.
func msdSort(a []string, lo, hi, d int, aux []string) {
	if hi <= lo+insertCutoff {
		insertion(a, lo, hi, d)
		return
	}

	// Key-indexed counting on byte d, with the -1 end-of-key bucket
	// sorted before byte 0 (counts shifted by two: index c+2 for
	// counting, c+1 for the distributed position).
	var count [R + 2]int
	for i := lo; i <= hi; i++ {
		count[byteAt(a[i], d)+2]++
	}
	for r := 0; r < R+1; r++ {
		count[r+1] += count[r]
	}
	for i := lo; i <= hi; i++ {
		c := byteAt(a[i], d)
		aux[count[c+1]] = a[i]
		count[c+1]++
	}
	for i := lo; i <= hi; i++ {
		a[i] = aux[i-lo]
	}

	// Recurse inside each byte's bucket.  The end-of-key bucket at the
	// front is already fully sorted — its keys all end here, and since
	// they share d bytes they are equal — so it is skipped (algs4
	// recurses on byte values 0..R-1 only, for the same reason).
	// Skipping buckets known empty is a no-op-elimination only.
	for r := 0; r < R; r++ {
		if count[r] < count[r+1] {
			msdSort(a, lo+count[r], lo+count[r+1]-1, d+1, aux)
		}
	}
}
