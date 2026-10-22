/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st

import "slices"

// Floor returns the entry with the largest key that compares <= key.
// found is false when every key sorts after key (or the table is
// empty).  As with Get, `key` only needs the fields that the table's
// comparison function reads.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Floor(key K) (k K, val V, found bool) {
	if st == nil || len(st.keys) == 0 {
		return
	}
	i := st.search(key)
	if i < len(st.keys) && st.cmp(st.keys[i], key) == 0 {
		return st.keys[i], st.vals[i], true
	}
	if i > 0 {
		return st.keys[i-1], st.vals[i-1], true
	}
	return
}

// Ceil returns the entry with the smallest key that compares >= key.
// found is false when every key sorts before key (or the table is
// empty).
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Ceil(key K) (k K, val V, found bool) {
	if st == nil || len(st.keys) == 0 {
		return
	}
	i := st.search(key)
	if i < len(st.keys) {
		return st.keys[i], st.vals[i], true
	}
	return
}

// Rank returns the 0-based position of key in ascending order — the
// number of keys that sort before it (AtIndex is its inverse).  found
// is false when no key compares equal to key; the rank of a missing
// key is not defined and 0 is returned with it, the skip_list
// convention.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Rank(key K) (rank int, found bool) {
	if st == nil || len(st.keys) == 0 {
		return
	}
	i := st.search(key)
	if i < len(st.keys) && st.cmp(st.keys[i], key) == 0 {
		return i, true
	}
	return 0, false
}

// AtIndex returns the entry at 0-based position i in ascending order —
// AtIndex(0) is FindMin, AtIndex(Len()-1) is FindMax.  found is false
// when i is outside [0, Len()).
// Complexity is O(1).
func (st *BinarySearchST[K, V]) AtIndex(i int) (key K, val V, found bool) {
	if st == nil || i < 0 || i >= len(st.keys) {
		return
	}
	return st.keys[i], st.vals[i], true
}

// span returns the half-open index range [a, b) of the entries whose
// keys compare within the closed range [lo, hi]; b == a when nothing
// is in range.  The caller is responsible for the empty/nil and
// lo-after-hi guards.
func (st *BinarySearchST[K, V]) span(lo, hi K) (a, b int) {
	a = st.search(lo)
	b = st.search(hi)
	if b < len(st.keys) && st.cmp(st.keys[b], hi) == 0 {
		b++ // the upper bound itself is present — include it
	}
	return a, b
}

// inRange reports whether a range query has anything to look at: a
// non-empty table with lo sorting before (or equal to) hi.
func (st *BinarySearchST[K, V]) inRange(lo, hi K) bool {
	return st != nil && len(st.keys) > 0 && st.cmp(lo, hi) <= 0
}

// CountRange returns the number of keys that compare within the closed
// range [lo, hi]; 0 when lo sorts after hi.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) CountRange(lo, hi K) int {
	if !st.inRange(lo, hi) {
		return 0
	}
	a, b := st.span(lo, hi)
	return b - a
}

// DeleteRange removes every key that compares within the closed range
// [lo, hi] and returns how many entries were removed; 0 when lo sorts
// after hi.
// Complexity is O(log₂ n + n) — one splice of the whole span.
func (st *BinarySearchST[K, V]) DeleteRange(lo, hi K) (deleted int) {
	if !st.inRange(lo, hi) {
		return 0
	}
	a, b := st.span(lo, hi)
	st.keys = slices.Delete(st.keys, a, b)
	st.vals = slices.Delete(st.vals, a, b)
	return b - a
}
