/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow

import "encoding/json"

// MarshalJSON implements json.Marshaler so a HashTab can be used directly
// with the encoding/json package.  The table is encoded as a JSON array of
// its elements in bucket order (the order Walk, All and Values use).  Bucket
// order depends on the per-table hash seed, so the array order varies from
// process to process — a hash table is a set, and the round trip preserves
// membership, not order.
//
// The elements are encoded by the json package itself, so a T with its own
// MarshalJSON — or struct field tags — is honored; only the table structure
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
		return []byte("[]"), nil
	}
	items := make([]T, 0, tt.length)
	for i := range tt.buckets {
		if tt.originalHash[i] != 0 {
			items = append(items, tt.buckets[i])
		}
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a HashTab can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the table.
// The equality and hash functions are kept, so the table stays usable after
// unmarshaling.  Equal elements in the array collapse the way repeated
// Insert calls do — the last one wins.
//
// The elements are decoded by the json package itself, so a T with its own
// UnmarshalJSON — or struct field tags — is honored.  The decode runs
// before anything is mutated: a decode error (malformed JSON, a non-array
// document, wrong element types) is returned and leaves the table untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data that
// would store an element into a nil table or a zero-value table (no
// equality/hash functions) panics with the standard insert-family message.
// An empty array or null clears the table and is tolerated everywhere — it
// stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (tt *HashTab[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("hash_grow: UnmarshalJSON called on a nil table")
		}
		if tt.eq == nil || tt.hash == nil {
			panic("hash_grow: UnmarshalJSON called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
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
