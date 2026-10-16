/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package string_sort implements the character-key radix sorts from
// Sedgwick & Wayne, Algorithms, 4th ed. §5.1 — LSD.java, MSD.java and
// Quick3string.java — as stateless in-place functions over []string:
//
//	LSDSort(a, w)   w passes of key-indexed counting, least significant
//	                byte first (fixed-width keys)
//	MSDSort(a)      most-significant-byte-first radix sort, recursive
//	Quick3String(a) 3-way radix quicksort, in place
//
// Keys order byte-wise lexicographically — exactly Go's native string
// < — so any Go string sorts correctly (binary-safe and UTF-8 alike;
// the ordering is by bytes, not runes, the same as slices.Sort), and a
// key that is a prefix of another sorts first.  All three sorts are
// byte-oriented with a 256-bucket alphabet, one bucket per byte value
// (the algs4 extended-ASCII assumption covers every Go string), and
// never compare through an interface.
//
// LSDSort and MSDSort are stable (equal keys keep their input order);
// Quick3String is not.  LSDSort and MSDSort allocate one O(n)
// auxiliary slice; Quick3String is fully in place.
//
// Nil, empty and single-element slices are no-ops for all three
// functions.  The package panics in exactly two situations, both
// programmer errors in LSDSort — the only one of the three with a
// precondition (it reads byte positions 0..w-1 of every key):
//
//	LSDSort(a, w) with w < 0              — negative key width.
//	LSDSort(a, w) with a key shorter      — a fixed-width precondition
//	than w                                  violation.
//
// The functions are stateless and safe for concurrent use on disjoint
// slices; there is no shared package state, so no _ts twin is needed
// (the quick_sort/geo/bitmap precedent).
package string_sort

// R is the radix of all three sorts: one bucket per byte value.  This
// is algs4's extended-ASCII alphabet, which in Go covers every string
// (binary-safe keys included).
const R = 256

// insertCutoff is the subarray length at or below which the recursive
// sorts (MSDSort, Quick3String) finish with an insertion sort on the
// remaining bytes — algs4's CUTOFF = 15.  Insertion sort is faster on
// small subarrays and, critically for MSDSort, stops the recursion
// from shrinking one key at a time down a shared prefix.
const insertCutoff = 15

// byteAt returns the byte at position d of s, or -1 when d is past the
// end of the key — the MSD/Quick3string end-of-key sentinel.  Because
// -1 sorts below every byte value, a key that is a prefix of another
// compares first, matching Go's native string ordering.
func byteAt(s string, d int) int {
	if d < len(s) {
		return int(s[d])
	}
	return -1
}

// less reports whether v sorts before w comparing from byte position d
// on.  Callers guarantee that v and w share their first d bytes (both
// recursive sorts only ever compare inside a band that has already
// matched d bytes), so comparing the remainders is the full
// comparison; Go's string < is byte-wise with the shorter prefix
// first, which is exactly algs4's less(v, w, d) loop including its
// end-of-key rule.
func less(v, w string, d int) bool {
	return v[d:] < w[d:]
}

// insertion sorts a[lo..hi] by the bytes from position d on — the
// shared cutoff subroutine of MSD.java and Quick3string.java.  All
// keys in the band share their first d bytes.  Complexity is O(m²)
// byte comparisons for a band of m keys.
func insertion(a []string, lo, hi, d int) {
	for i := lo + 1; i <= hi; i++ {
		for j := i; j > lo && less(a[j], a[j-1], d); j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
