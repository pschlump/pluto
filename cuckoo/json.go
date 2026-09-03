/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo

import "encoding/json"

// MarshalJSON implements json.Marshaler so a HashTab can be used directly
// with the encoding/json package.  The table is encoded as a JSON array of
// its elements, in slot order (the order Walk, All and Values use).  Slot
// order depends on the hash seed and the displacement history, so the output
// ordering is not stable across processes — decode and sort when order
// matters.
//
// The elements are encoded by the json package itself, so a T with its own
// MarshalJSON — or struct field tags — is honored; only the array structure
// is pluto's.  Errors from the json package are returned unchanged (for
// example a T that cannot be encoded at all, such as a channel or a
// function).
//
// An empty table encodes as [].  A direct call on a nil table also encodes
// as [] (the "nil behaves as an empty table" read contract); note that
// json.Marshal on a nil *HashTab never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *HashTab[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil // a nil table marshals as an empty array
	}
	items := make([]T, 0, tt.length) // non-nil, so an empty table encodes as []
	for i := range tt.slots {
		if tt.slots[i].used {
			items = append(items, tt.slots[i].data)
		}
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a HashTab can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the table.
// The equality and hash functions are kept, so the table stays usable after
// unmarshaling — every decoded element is hashed and placed by the same
// rules as Insert, and a duplicate within data settles as the last
// occurrence.
//
// The elements are decoded by the json package itself, so a T with its own
// UnmarshalJSON — or struct field tags — is honored.  A decode error
// (malformed JSON, a non-array document, wrong element types) is returned
// and leaves the table untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data that
// would store an element into a nil table or a zero-value table (no
// equality/hash functions) panics with the standard insert-family message.
// An empty array or null clears the table and is tolerated everywhere — it
// stores nothing.
// Complexity is O(n) plus the cost of decoding the elements; the placement
// of the decoded elements follows the Insert complexity, O(1) average each.
func (tt *HashTab[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Concat precedent).
	if len(items) > 0 {
		if tt == nil {
			panic("cuckoo: UnmarshalJSON called on a nil table")
		}
		if tt.eq == nil || tt.hash == nil {
			panic("cuckoo: UnmarshalJSON called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.Truncate()
	for _, d := range items {
		tt.Insert(d)
	}
	return nil
}
