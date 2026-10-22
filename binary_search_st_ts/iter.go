/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st_ts

import "iter"

// The iterators are SNAPSHOT iterators, the twins' rule: the entries
// are copied under the read lock when iteration starts and the copy is
// yielded, so the table may be freely mutated — even from inside the
// loop body.  (The plain package's iterators walk the captured backing
// arrays and do not cover mutation.)

// All returns an iterator over the entries in ascending key order, as
// (key, value) pairs, on a snapshot.  Iterating an empty or nil table
// yields nothing.
// Complexity is O(n) for the snapshot plus O(n) to walk it.
func (st *BinarySearchST[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, e := range st.entries() { // snapshot under the read lock
			if !yield(e.k, e.v) {
				return
			}
		}
	}
}

// Backward returns an iterator over the entries in descending key
// order, as (key, value) pairs, on a snapshot.  Iterating an empty or
// nil table yields nothing.
// Complexity is O(n) for the snapshot plus O(n) to walk it.
func (st *BinarySearchST[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		entries := st.entries() // snapshot under the read lock
		for i := len(entries) - 1; i >= 0; i-- {
			if !yield(entries[i].k, entries[i].v) {
				return
			}
		}
	}
}

// Range returns an iterator over the entries whose keys compare within
// the closed range [lo, hi], in ascending order, on a snapshot.  A
// range with lo sorting after hi yields nothing.
// Complexity is O(log₂ n) for the snapshot span plus O(m) to walk it,
// m = entries in range.
func (st *BinarySearchST[K, V]) Range(lo, hi K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, e := range st.spanEntries(lo, hi) { // snapshot under the read lock
			if !yield(e.k, e.v) {
				return
			}
		}
	}
}

// kv is one copied entry of a snapshot.
type kv[K, V any] struct {
	k K
	v V
}

// entries copies the whole table under the read lock — the snapshot
// helper of All, Backward, DeleteFunc and StringFunc.  A nil table
// yields nil.
func (st *BinarySearchST[K, V]) entries() []kv[K, V] {
	if st == nil {
		return nil
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	out := make([]kv[K, V], 0, st.inner.Len())
	for k, v := range st.inner.All() {
		out = append(out, kv[K, V]{k: k, v: v})
	}
	return out
}

// spanEntries copies the entries in the closed range [lo, hi] under
// the read lock — Range's snapshot helper.  A nil table yields nil.
func (st *BinarySearchST[K, V]) spanEntries(lo, hi K) []kv[K, V] {
	if st == nil {
		return nil
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	var out []kv[K, V]
	for k, v := range st.inner.Range(lo, hi) {
		out = append(out, kv[K, V]{k: k, v: v})
	}
	return out
}
