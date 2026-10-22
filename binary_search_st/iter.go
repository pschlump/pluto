/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st

import "iter"

// All returns an iterator over the entries in ascending key order, as
// (key, value) pairs.  Iterating an empty or nil table yields nothing.
// The arrays are captured when iteration starts; the contract does not
// cover mutating the table while iterating.
// Complexity is O(n) to walk, O(1) extra space.
func (st *BinarySearchST[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if st == nil {
			return
		}
		keys, vals := st.keys, st.vals
		for i := range keys {
			if !yield(keys[i], vals[i]) {
				return
			}
		}
	}
}

// Backward returns an iterator over the entries in descending key
// order, as (key, value) pairs.  Iterating an empty or nil table
// yields nothing.  The arrays are captured when iteration starts; the
// contract does not cover mutating the table while iterating.
// Complexity is O(n) to walk, O(1) extra space.
func (st *BinarySearchST[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if st == nil {
			return
		}
		keys, vals := st.keys, st.vals
		for i := len(keys) - 1; i >= 0; i-- {
			if !yield(keys[i], vals[i]) {
				return
			}
		}
	}
}

// Range returns an iterator over the entries whose keys compare within
// the closed range [lo, hi], in ascending order, as (key, value)
// pairs.  A range with lo sorting after hi yields nothing.  The arrays
// are captured when iteration starts; the contract does not cover
// mutating the table while iterating.
// Complexity is O(log₂ n + m), m = entries in range.
func (st *BinarySearchST[K, V]) Range(lo, hi K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if !st.inRange(lo, hi) {
			return
		}
		a, b := st.span(lo, hi)
		for i := a; i < b; i++ {
			if !yield(st.keys[i], st.vals[i]) {
				return
			}
		}
	}
}
