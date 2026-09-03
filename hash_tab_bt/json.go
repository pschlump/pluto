/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_tab_bt

import "encoding/json"

// MarshalJSON implements json.Marshaler so a HashTab can be used directly
// with the encoding/json package.  The table is encoded as a JSON array
// of its elements, in the table's iteration order: bucket order and,
// within a bucket, the tree's in-order (ascending per the comparison
// function).  Bucket order depends on the hash function and, for tables
// created with NewHashTab, on the per-table random seed, so the element
// order in the output is not stable — never rely on it.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the table
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty table encodes as [].  A direct call on a nil table also
// encodes as [] (the "nil behaves as an empty table" read contract);
// note that json.Marshal on a nil *HashTab never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *HashTab[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, tt.length)
	for item := range tt.Values() {
		items = append(items, item)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a HashTab can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// table.  Duplicate elements in the array collapse the way duplicate
// inserts do: the last one wins.  The comparison and hash functions are
// kept, so the table stays usable after unmarshaling.  Element order in
// the array does not matter — the elements land wherever the hash
// function puts them.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// runs before anything is touched, so a decode error (malformed JSON, a
// non-array document, wrong element types) is returned with the table
// untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil table or a zero-value table (no
// comparison/hash functions) panics with the standard insert-family
// message.  An empty array or null clears the table and is tolerated
// everywhere — it stores nothing.
// Complexity is n inserts into the bucket trees, O(log(n/k)) average
// each, plus the cost of decoding the elements.
func (tt *HashTab[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the dll Concat precedent).
	if len(items) > 0 {
		if tt == nil {
			panic("hash_tab_bt: UnmarshalJSON called on a nil table")
		}
		if tt.cmp == nil || tt.hash == nil {
			panic("hash_tab_bt: UnmarshalJSON called on a table with no comparison/hash functions (create the table with NewHashTab or NewHashTabFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.Truncate()
	for _, item := range items {
		tt.Insert(item)
	}
	return nil
}
