/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st_ts

// The order statistics riders — the skip_list names and semantics,
// delegated to the plain package under the appropriate lock.  Rank is
// in skip_list's present-key semantics: the position the key would
// occupy, and whether it is actually present.

// Floor returns the greatest key less than or equal to key, with its
// value.  It reports false when every key sorts after key (an empty
// table included).
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Floor(key K) (k K, val V, found bool) {
	if st == nil {
		return
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Floor(key)
}

// Ceil returns the least key greater than or equal to key, with its
// value.  It reports false when every key sorts before key (an empty
// table included).
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Ceil(key K) (k K, val V, found bool) {
	if st == nil {
		return
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Ceil(key)
}

// Rank returns the 0-based position of key in ascending order — the
// number of keys that sort before it (AtIndex is its inverse).  found
// is false when no key compares equal to key; the rank of a missing
// key is not defined and 0 is returned with it, the skip_list
// convention.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) Rank(key K) (rank int, found bool) {
	if st == nil {
		return 0, false
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.Rank(key)
}

// AtIndex returns the key and value at position i in ascending key
// order (0-based).  It reports false when i is out of range
// [0, Len()).
// Complexity is O(1).
func (st *BinarySearchST[K, V]) AtIndex(i int) (key K, val V, found bool) {
	if st == nil {
		return
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.AtIndex(i)
}

// CountRange returns the number of keys in the closed range [lo, hi].
// A range with lo sorting after hi counts zero.
// Complexity is O(log₂ n).
func (st *BinarySearchST[K, V]) CountRange(lo, hi K) int {
	if st == nil {
		return 0
	}
	st.lock.RLock()
	defer st.lock.RUnlock()
	return st.inner.CountRange(lo, hi)
}

// DeleteRange removes every entry whose key sorts within the closed
// range [lo, hi] and returns how many were removed.  A range with lo
// sorting after hi deletes nothing.
// Complexity is O(n).
func (st *BinarySearchST[K, V]) DeleteRange(lo, hi K) (deleted int) {
	if st == nil {
		return 0
	}
	st.lock.Lock()
	defer st.lock.Unlock()
	return st.inner.DeleteRange(lo, hi)
}
