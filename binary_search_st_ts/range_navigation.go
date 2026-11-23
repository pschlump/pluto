/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/
package binary_search_st_ts

import (
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// Lower returns the entry with the largest key strictly less than key.
func (st *BinarySearchST[K, V]) Lower(key K) (K, V, bool) {
	if st == nil {
		return *new(K), *new(V), false
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Lower(key)
}

// Higher returns the entry with the smallest key strictly greater than key.
func (st *BinarySearchST[K, V]) Higher(key K) (K, V, bool) {
	if st == nil {
		return *new(K), *new(V), false
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Higher(key)
}

// RangeOf returns a snapshot iterator over an open/closed/unbounded range.
func (st *BinarySearchST[K, V]) RangeOf(value ranges.Range[K]) iter.Seq2[K, V] {
	if st == nil {
		return func(func(K, V) bool) {}
	}
	st.lock.RLock()
	entries := []kv[K, V]{}
	for key, item := range st.inner.RangeOf(value) {
		entries = append(entries, kv[K, V]{k: key, v: item})
	}
	st.lock.RUnlock()
	return func(yield func(K, V) bool) {
		for _, entry := range entries {
			if !yield(entry.k, entry.v) {
				return
			}
		}
	}
}
