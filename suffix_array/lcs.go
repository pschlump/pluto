/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package suffix_array

// LCS returns the longest common substring of s and t (Sedgwick's
// LongestCommonSubstring client, §6.3).  Ties are broken by first in
// sorted order, like LongestRepeatedSubstring.  When s and t share no
// substring the answer is ("", true).
//
// The construction is the algs4 one: build the suffix array of
// s+sep+t, then scan adjacent sorted suffixes for the longest common
// prefix of a pair from opposite halves — the maximum common substring
// always shows up on some adjacent cross-half pair, because any suffix
// sorted between two suffixes sharing a prefix of length L shares at
// least that prefix with both.
//
// The separator sep is any byte value that appears in neither string
// (algs4 hardcodes '#'; pluto picks one automatically), so the halves
// can be told apart and no suffix of s can run on into t.  ok is false
// only in the pathological case that s and t together use all 256 byte
// values, leaving no separator to build with.
// Complexity is O(n log² n) for the suffix array plus O(n) for the
// scan, where n = len(s) + len(t).
func LCS(s, t string) (lcs string, ok bool) {
	sep, ok := pickSeparator(s, t)
	if !ok {
		return "", false
	}
	ns := len(s)
	cat := s + string(sep) + t
	sa := NewSuffixArray(cat)
	bestOff, bestLen := 0, 0
	for i := 1; i < sa.Len(); i++ {
		l, _ := sa.LCP(i)
		if l <= bestLen {
			continue
		}
		a, _ := sa.Index(i)
		b, _ := sa.Index(i - 1)
		if (a < ns && b > ns) || (b < ns && a > ns) {
			bestOff, bestLen = a, l
		}
	}
	if bestLen == 0 {
		return "", true
	}
	return cat[bestOff : bestOff+bestLen], true
}

// pickSeparator returns a byte value that appears in neither s nor t.
func pickSeparator(s, t string) (sep byte, ok bool) {
	var used [256]bool
	for i := 0; i < len(s); i++ {
		used[s[i]] = true
	}
	for i := 0; i < len(t); i++ {
		used[t[i]] = true
	}
	for b := 0; b < 256; b++ {
		if !used[b] {
			return byte(b), true
		}
	}
	return 0, false
}
