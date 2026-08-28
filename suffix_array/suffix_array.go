/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package suffix_array implements the suffix array with LCP (longest
// common prefix) of Sedgwick & Wayne, Algorithms, 4th ed. §6.3.
//
// A suffix array of a string s of length n is the sorted order of s's
// n suffixes: suffixes[i] is the text offset of the i-th smallest
// suffix.  Alongside it the constructor builds the LCP table (lcp[i]
// is the length of the longest common prefix of the sorted suffixes i
// and i-1, with lcp[0] == 0) and the rank table (rank[i] is the
// position of suffix i in the sorted order — the inverse permutation),
// which together answer longest-repeated-substring-style queries.
//
// Like union_find the structure is non-generic: there is no element
// type T and no comparison function to supply, because the
// lexicographic order is inherent in the bytes of the string.
//
// Basic operations:
//
//	NewSuffixArray(s) — build the suffix array, LCP and rank tables.	O(n log² n)
//	Len — the number of suffixes (== len(s)).							O(1)
//	Index(i) — text offset of the i-th sorted suffix.					O(1)
//	Select(i) — the i-th sorted suffix itself.							O(1)
//	LCP(i) — longest common prefix of sorted suffixes i and i-1.		O(1)
//	Rank(i) — position of suffix i in the sorted order.					O(1)
//	LongestRepeatedSubstring — longest substring appearing ≥ 2 times.	O(n)
//
// Construction sorts the suffixes by prefix doubling: starting from
// byte-value ranks, the indices are repeatedly sorted by the pair
// (rank[i], rank[i+k] or -1 when out of range) for k = 1, 2, 4, ...
// until every rank is distinct.  The LCP table is then built with
// Kasai's algorithm in O(n).
//
// The structure is IMMUTABLE after construction: NewSuffixArray builds
// every table and nothing ever mutates them afterwards.
//
// The package NEVER panics.  A nil *SuffixArray and the zero value
// behave as an empty suffix array for every operation: Len returns 0,
// Index/Select/LCP/Rank return (0, false) / ("", false), and
// LongestRepeatedSubstring returns "".
//
// Because the structure is immutable, it is SAFE FOR CONCURRENT USE by
// multiple readers (indeed by anyone — there is nothing to mutate), so
// there is no mutex-guarded _ts twin.
package suffix_array

import "sort"

// SuffixArray is the suffix array of a string, together with its LCP
// and rank tables.  It is built by NewSuffixArray and is immutable
// thereafter.  The zero value is an empty suffix array.
type SuffixArray struct {
	text     string // the string the suffixes were taken from
	suffixes []int  // suffixes[i] is the text offset of the i-th suffix in sorted order
	lcp      []int  // lcp[i] is the LCP of sorted suffixes i and i-1; lcp[0] == 0
	rank     []int  // rank[i] is the position of suffix i in the sorted order
}

// NewSuffixArray returns the suffix array of s, with its LCP and rank
// tables.  The empty string is valid input (Len() == 0).
//
// The suffixes are sorted by prefix doubling — O(n log² n) — and the
// LCP table is built with Kasai's algorithm in O(n).
// Complexity is O(n log² n), where n = len(s).
func NewSuffixArray(s string) *SuffixArray {
	n := len(s)
	sa := &SuffixArray{text: s}
	if n == 0 {
		return sa
	}

	suffixes := make([]int, n)
	rank := make([]int, n) // ranks of length-k prefixes; initially the byte values
	for i := range n {
		suffixes[i] = i
		rank[i] = int(s[i])
	}
	tmp := make([]int, n)
	for k := 1; ; k *= 2 {
		// Sort by the pair (rank[i], rank[i+k] or -1): given rank[] for
		// the first k bytes, this orders by the first 2k bytes.
		sort.Slice(suffixes, func(x, y int) bool {
			i, j := suffixes[x], suffixes[y]
			if rank[i] != rank[j] {
				return rank[i] < rank[j]
			}
			return secondRank(rank, i, k) < secondRank(rank, j, k)
		})
		// Re-rank: assign fresh classes 0..classes to the sorted suffixes.
		classes := 0
		for p, sfx := range suffixes {
			if p > 0 {
				prev := suffixes[p-1]
				if rank[prev] != rank[sfx] || secondRank(rank, prev, k) != secondRank(rank, sfx, k) {
					classes++
				}
			}
			tmp[sfx] = classes
		}
		rank, tmp = tmp, rank
		if classes == n-1 {
			break // all ranks distinct: the order is final
		}
	}
	sa.suffixes = suffixes
	sa.rank = rank

	// Kasai's algorithm: lcp[rank[i]] is computed in text order of i,
	// reusing the previous suffix's LCP minus one as a starting point.
	sa.lcp = make([]int, n)
	h := 0
	for i := 0; i < n; i++ {
		r := rank[i]
		if r == 0 {
			continue
		}
		j := suffixes[r-1]
		for i+h < n && j+h < n && s[i+h] == s[j+h] {
			h++
		}
		sa.lcp[r] = h
		if h > 0 {
			h--
		}
	}
	return sa
}

// secondRank is the second half of the doubling pair for suffix i: the
// rank of suffix i+k, or -1 when i+k is out of range (a suffix that
// ends first sorts first).
func secondRank(rank []int, i, k int) int {
	if i+k < len(rank) {
		return rank[i+k]
	}
	return -1
}

// inRange reports whether i is a valid sorted-order position of sa.
func (sa *SuffixArray) inRange(i int) bool {
	return sa != nil && i >= 0 && i < len(sa.suffixes)
}

// Len returns the number of suffixes, which is len(s) for the s the
// array was built from.  A nil or zero-value suffix array reports 0.
// Complexity is O(1).
func (sa *SuffixArray) Len() int {
	if sa == nil {
		return 0
	}
	return len(sa.suffixes)
}

// Index returns the text offset of the i-th suffix in sorted order.
// ok is false — and the offset is 0 — if i is out of range or sa is
// nil or a zero value.
// Complexity is O(1).
func (sa *SuffixArray) Index(i int) (offset int, ok bool) {
	if !sa.inRange(i) {
		return 0, false
	}
	return sa.suffixes[i], true
}

// Select returns the i-th suffix in sorted order.  ok is false — and
// the suffix is "" — if i is out of range or sa is nil or a zero
// value.
// Complexity is O(1).
func (sa *SuffixArray) Select(i int) (suffix string, ok bool) {
	if !sa.inRange(i) {
		return "", false
	}
	return sa.text[sa.suffixes[i]:], true
}

// LCP returns the length of the longest common prefix of the i-th and
// (i-1)-th suffixes in sorted order; it is 0 for i == 0.  ok is false
// — and the length is 0 — if i is out of range or sa is nil or a zero
// value.
// Complexity is O(1).
func (sa *SuffixArray) LCP(i int) (length int, ok bool) {
	if !sa.inRange(i) {
		return 0, false
	}
	return sa.lcp[i], true
}

// Rank returns the position of suffix i (the suffix starting at text
// offset i) in the sorted order.  ok is false — and the rank is 0 —
// if i is out of range or sa is nil or a zero value.
// Complexity is O(1).
func (sa *SuffixArray) Rank(i int) (position int, ok bool) {
	if !sa.inRange(i) {
		return 0, false
	}
	return sa.rank[i], true
}

// LongestRepeatedSubstring returns the longest substring of the text
// that appears at least twice, or "" if there is no repeated
// substring.  It is the longest common prefix of some pair of adjacent
// sorted suffixes — the maximum over the LCP table, ties broken by
// first in sorted order.  A nil or zero-value suffix array reports "".
// Complexity is O(n).
func (sa *SuffixArray) LongestRepeatedSubstring() string {
	if sa == nil {
		return ""
	}
	best := ""
	for i := 1; i < len(sa.lcp); i++ {
		if sa.lcp[i] > len(best) {
			best = sa.text[sa.suffixes[i] : sa.suffixes[i]+sa.lcp[i]]
		}
	}
	return best
}
