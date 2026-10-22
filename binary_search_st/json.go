/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st

import (
	"encoding/json"
)

// jsonPair is the wire form of one (key, value) entry of the table,
// the index_pq shape.
type jsonPair[K, V any] struct {
	K K `json:"k"`
	V V `json:"v"`
}

// MarshalJSON implements json.Marshaler so a BinarySearchST can be
// used directly with the encoding/json package.  The table is encoded
// as a JSON array of {"k":key,"v":value} objects in ascending key
// order — the same order All yields.
//
// The keys and values are encoded by the json package itself, so a K
// or V with its own MarshalJSON — or struct field tags — is honored;
// only the table structure is pluto's.  Errors from the json package
// are returned unchanged (for example a key type that cannot be
// encoded at all, such as a channel or a function).
//
// An empty table encodes as [].  A direct call on a nil table also
// encodes as [] (the "nil behaves as an empty table" read contract);
// note that json.Marshal on a nil *BinarySearchST never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the keys and values.
func (st *BinarySearchST[K, V]) MarshalJSON() ([]byte, error) {
	if st == nil {
		return []byte("[]"), nil
	}
	pairs := make([]jsonPair[K, V], 0, len(st.keys))
	for i := range st.keys {
		pairs = append(pairs, jsonPair[K, V]{K: st.keys[i], V: st.vals[i]})
	}
	return json.Marshal(pairs)
}

// UnmarshalJSON implements json.Unmarshaler so a BinarySearchST can be
// used directly with the encoding/json package.  data must be a JSON
// array of {"k":key,"v":value} objects (or null); the decoded pairs
// replace the current contents of the table — duplicate keys follow
// the Put convention, so the last pair for a key wins.  The comparison
// function is kept, so the table stays usable after unmarshaling.
//
// The keys and values are decoded by the json package itself, so a K
// or V with its own UnmarshalJSON — or struct field tags — is honored.
// The whole document is decoded before anything is stored: a decode
// error (malformed JSON, a non-array document, wrong key or value
// types) is returned and leaves the table untouched.  data does not
// have to be sorted; the pairs are Put in order and land sorted.
//
// Unmarshaling stores entries, so it follows the Put contract: data
// that would store a pair into a nil table or a zero-value table (no
// comparison function) panics with the standard Put-family message.
// An empty array or null clears the table and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n·(log₂ n + n)) for the ordered inserts, plus the
// cost of decoding the keys and values.
func (st *BinarySearchST[K, V]) UnmarshalJSON(data []byte) error {
	var pairs []jsonPair[K, V]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The Put contract only fires when a pair would actually be stored
	// (the index_pq precedent).
	if len(pairs) > 0 {
		if st == nil {
			panic("binary_search_st: UnmarshalJSON called on a nil symbol table (create the table with NewBinarySearchST or NewBinarySearchSTFunc)")
		}
		if st.cmp == nil {
			panic("binary_search_st: UnmarshalJSON called on a zero-value symbol table with no comparison function (create the table with NewBinarySearchST or NewBinarySearchSTFunc)")
		}
	}
	if st == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	st.Truncate()
	for _, p := range pairs {
		st.Put(p.K, p.V) // duplicates: the last pair for a key wins
	}
	return nil
}
