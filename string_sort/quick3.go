/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package string_sort

// Quick3String sorts variable-width string keys in place with 3-way
// radix quicksort — algs4 §5.1 Quick3string.java: partition the band
// into bytes less than, equal to and greater than a pivot byte at the
// current position, recurse the two side bands at the same position,
// and recurse the equal band one byte deeper.  Compared with MSDSort
// it re-examines bytes on each comparison but does no 256-bucket
// counting pass, so it stays fast on long shared prefixes without the
// per-bucket overhead.
//
// Two deviations from the algs4 reference, both shared with pluto's
// quick_sort (a library cannot pre-shuffle the input the way the
// textbook client does): the pivot byte is the median of the bytes at
// the two ends and the middle of the band (algs4 uses the first key's
// byte), and the sort recurses into the smaller of the two side bands
// while looping on the larger, keeping the partition stack O(log n).
// Subarrays of insertCutoff or fewer keys switch to an insertion sort
// on the remaining bytes.
//
// The sort is NOT stable.  Nil, empty and single-element slices are
// no-ops.  There are no panics.
// Complexity is O(n log n) expected byte examinations on random keys
// (O(n·w) worst case); extra space is O(1) apart from the recursion
// stack, whose depth is O(log n) plus the length of the longest shared
// prefix.
func Quick3String(a []string) {
	q3sort(a, 0, len(a)-1, 0)
}

// q3sort sorts a[lo..hi] — keys known to share their first d bytes —
// on the bytes from position d on.  This is the body of algs4
// Quick3string.sort with the two pluto deviations (median-of-3 pivot,
// smaller-side recursion).
func q3sort(a []string, lo, hi, d int) {
	for hi > lo+insertCutoff {
		// 3-way partition on the pivot byte v, leaving
		// a[lo..lt-1] < v, a[lt..gt] == v, a[gt+1..hi] > v.
		lt, i, gt := lo, lo, hi
		v := med3(byteAt(a[lo], d), byteAt(a[lo+(hi-lo)/2], d), byteAt(a[hi], d))
		for i <= gt {
			t := byteAt(a[i], d)
			switch {
			case t < v:
				a[lt], a[i] = a[i], a[lt]
				lt++
				i++
			case t > v:
				a[i], a[gt] = a[gt], a[i]
				gt--
			default:
				i++
			}
		}

		// The equal band shares one more byte, so it sorts at d+1 —
		// unless the pivot was the end-of-key sentinel: those keys all
		// end here and, sharing d bytes, are equal and already done.
		if v >= 0 {
			q3sort(a, lt, gt, d+1)
		}

		// Recurse into the smaller of the two side bands and loop on
		// the larger — the quick_sort O(log n) stack discipline.
		if lt-lo < hi-gt {
			q3sort(a, lo, lt-1, d)
			lo = gt + 1
		} else {
			q3sort(a, gt+1, hi, d)
			hi = lt - 1
		}
	}
	insertion(a, lo, hi, d)
}

// med3 returns the median of three ints — the pivot byte selector.
func med3(a, b, c int) int {
	if a < b {
		if b < c {
			return b
		}
		if a < c {
			return c
		}
		return a
	}
	if a < c {
		return a
	}
	if b < c {
		return c
	}
	return b
}
