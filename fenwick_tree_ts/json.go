/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package fenwick_tree_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a FenwickTree can be used
// directly with the encoding/json package.  The tree is encoded as a
// JSON array of its per-index values: element i of the array is
// Value(i), so the encoding carries the values, not the internal
// 1-based prefix-sum array.
//
// The values are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any tree operation.  Each value is reconstructed as the
// difference of two prefix sums, so the snapshot costs O(n log n).
//
// A zero-value tree encodes as [].  A direct call on a nil tree also
// encodes as [] (the "nil behaves as an empty tree" read contract);
// note that json.Marshal on a nil *FenwickTree never reaches this
// method — the json package writes null for nil pointers itself.
// Errors from the json package are returned unchanged.
// Complexity is O(n log n) plus the cost of encoding the elements.
func (ft *FenwickTree[T]) MarshalJSON() ([]byte, error) {
	if ft == nil {
		return []byte("[]"), nil
	}
	ft.lock.RLock()
	n := ft.NlLen() // 0 for a zero-value tree (no slots)
	values := make([]T, 0, n)
	for i := 0; i < n; i++ {
		values = append(values, ft.nlValue(i))
	}
	ft.lock.RUnlock()
	return json.Marshal(values)
}

// UnmarshalJSON implements json.Unmarshaler so a FenwickTree can be
// used directly with the encoding/json package.  data must be a JSON
// array (or null); the decoded values replace the current contents of
// the tree — element i becomes Value(i) — under one hold of the write
// lock.  The tree is resized to the length of the array and rebuilt in
// O(n) by the same parent-range summation as NewFenwickTreeFrom; a
// Fenwick tree has no constructor-set functions, so a tree rebuilt this
// way — even from a zero value — stays fully usable.
//
// data is decoded before the lock is taken and a decode error
// (malformed JSON, a non-array document, wrong element types) is
// returned with the tree untouched.
//
// Unmarshaling stores values, so it follows the Add/Set contract: data
// that would store a value into a nil tree panics with the standard
// insert-family message.  An empty array or null clears the tree (to
// the empty, zero-slot state) and is tolerated everywhere — it stores
// nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (ft *FenwickTree[T]) UnmarshalJSON(data []byte) error {
	var values []T
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	// The insert contract only fires when a value would actually be
	// stored.
	if len(values) > 0 && ft == nil {
		panic("fenwick_tree_ts: UnmarshalJSON called on a nil FenwickTree")
	}
	if ft == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	// Build the 1-based internal array unlocked; only the swap needs
	// the write lock.  An empty array clears to the zero-slot state.
	n := len(values)
	tree := make([]T, n+1)
	copy(tree[1:], values)
	for i := 1; i <= n; i++ {
		if j := i + (i & -i); j <= n {
			tree[j] += tree[i]
		}
	}

	ft.lock.Lock()
	ft.tree = tree
	ft.lock.Unlock()
	return nil
}
