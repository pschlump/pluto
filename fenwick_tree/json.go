/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package fenwick_tree

import "encoding/json"

// MarshalJSON implements json.Marshaler so a FenwickTree can be used
// directly with the encoding/json package.  The tree is encoded as a
// JSON array of the per-index values: element 0 is Value(0), element 1
// is Value(1), and so on through Value(n-1) — the same numbers
// NewFenwickTreeFrom would take, not the internal 1-based sums.
//
// The values are encoded by the json package itself, so errors from it
// are returned unchanged.  (Every g_lib.Numeric type has a JSON
// encoding, so in practice only exotic shadows of the constraint can
// fail.)
//
// An empty tree — a nil *FenwickTree or the zero value — encodes as [];
// note that json.Marshal on a nil *FenwickTree never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n log n): each of the n values is the difference of
// two O(log n) prefix sums.
func (ft *FenwickTree[T]) MarshalJSON() ([]byte, error) {
	n := ft.Len() // 0 for a nil or zero-value tree
	values := make([]T, 0, n)
	for i := 0; i < n; i++ {
		values = append(values, ft.nlValue(i))
	}
	return json.Marshal(values)
}

// UnmarshalJSON implements json.Unmarshaler so a FenwickTree can be
// used directly with the encoding/json package.  data must be a JSON
// array (or null); the decoded values replace the current contents —
// element i becomes the new Value(i) — and the slot count becomes the
// array length, exactly as if NewFenwickTreeFrom had been called on the
// decoded slice.  The tree is rebuilt with the same O(n) bulk build as
// NewFenwickTreeFrom, so it stays fully usable after unmarshaling.
//
// A decode error (malformed JSON, a non-array document, wrong element
// types) is returned and leaves the tree untouched.
//
// Unmarshaling follows the package's write contract: data with elements
// panics on a nil *FenwickTree (a nil structure cannot record an
// update).  A zero-value tree is simply resized to fit — the tree holds
// no constructor-set state to lose.  An empty array or null clears the
// tree to zero slots and is tolerated everywhere — it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (ft *FenwickTree[T]) UnmarshalJSON(data []byte) error {
	var values []T
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	if len(values) > 0 && ft == nil {
		panic("fenwick_tree: UnmarshalJSON called on a nil FenwickTree")
	}
	if ft == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	n := len(values)
	tree := make([]T, n+1)
	copy(tree[1:], values)
	for i := 1; i <= n; i++ {
		if j := i + (i & -i); j <= n {
			tree[j] += tree[i]
		}
	}
	ft.tree = tree
	return nil
}
