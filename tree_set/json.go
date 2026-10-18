/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set

import "encoding/json"

// This file holds the JSON round-trip: a TreeSet marshals as a JSON
// array of its elements in ascending order, and UnmarshalJSON replaces
// the contents, keeping the comparison function.  The facade cannot
// delegate UnmarshalJSON — the wrapped tree would raise avl_tree's
// constructor-named panics — so the decode-then-rebuild is done here
// with tree_set's own contract messages, mirroring avl_tree's json.go
// exactly otherwise: a decode error leaves the set untouched, and the
// insert contract fires only when elements would actually be stored.

// MarshalJSON encodes the set as a JSON array of its elements in
// ascending order — the canonical form: two equal sets always encode
// byte for byte identically.  A nil set or a zero-value set encodes as
// an empty array.  (Caveat, as everywhere in pluto: encoding/json
// itself writes `null` for a nil *TreeSet passed through an interface,
// because json.Marshal checks for nil before calling MarshalJSON.)
// Complexity is O(n).
func (ts *TreeSet[T]) MarshalJSON() ([]byte, error) {
	if ts.innerOf() == nil {
		return []byte("[]"), nil
	}
	return ts.inner.MarshalJSON()
}

// UnmarshalJSON replaces the set's contents with the elements decoded
// from a JSON array, ordered by the set's existing comparison function;
// duplicates in the array collapse, exactly as inserting them in order
// would.  The comparison function is kept — a set that unmarshals still
// orders like the set it was before.  Storing into a nil set or a
// zero-value set (no comparison function) panics with the standard
// insert-family message; unmarshaling an empty array anywhere is
// tolerated and clears the set.  A decode error leaves the set
// untouched: the contents are replaced only after the whole array has
// decoded successfully.
// Complexity is O(k·log₂ k), k = number of decoded elements.
func (ts *TreeSet[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err // decode failed: the set is untouched
	}
	if len(items) == 0 {
		if ts != nil {
			ts.Truncate() // clearing is tolerated everywhere, keeps cmp
		}
		return nil
	}
	ts.insertContractCheck("UnmarshalJSON")
	ts.inner.Truncate() // replace, not append — and keep the tree's cmp
	ts.inner.InsertSlice(items)
	return nil
}
