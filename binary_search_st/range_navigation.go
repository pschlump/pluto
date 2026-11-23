/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package binary_search_st

import (
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the entry with the largest key strictly less than key.
func (st *BinarySearchST[K, V]) Lower(key K) (K, V, bool) {
	if st == nil {
		return *new(K), *new(V), false
	}
	position := st.search(key)
	if position == 0 {
		return *new(K), *new(V), false
	}
	return st.keys[position-1], st.vals[position-1], true
}

// Higher returns the entry with the smallest key strictly greater than key.
func (st *BinarySearchST[K, V]) Higher(key K) (K, V, bool) {
	if st == nil {
		return *new(K), *new(V), false
	}
	position := st.search(key)
	if position < len(st.keys) && st.cmp(st.keys[position], key) == 0 {
		position++
	}
	if position >= len(st.keys) {
		return *new(K), *new(V), false
	}
	return st.keys[position], st.vals[position], true
}

// RangeOf iterates over an open/closed/unbounded range in ascending key order.
// Complexity is O(log n + results).
func (st *BinarySearchST[K, V]) RangeOf(value ranges.Range[K]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if st == nil || len(st.keys) == 0 || value.IsEmpty() {
			return
		}
		lower, lowerBound, hasLower := value.LowerBound()
		upper, upperBound, hasUpper := value.UpperBound()
		start := 0
		if hasLower {
			start = st.search(lower)
			if start < len(st.keys) && lowerBound == ranges.OpenBound && st.cmp(st.keys[start], lower) == 0 {
				start++
			}
		}
		for index := start; index < len(st.keys); index++ {
			if hasUpper {
				order := st.cmp(st.keys[index], upper)
				if order > 0 || order == 0 && upperBound == ranges.OpenBound {
					return
				}
			}
			if !yield(st.keys[index], st.vals[index]) {
				return
			}
		}
	}
}
