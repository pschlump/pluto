/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_tab_bt_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a HashTab can be used directly
// with the encoding/json package.  The table is encoded as a JSON array of
// its elements in the natural iteration order — bucket order, ascending per
// the comparison function within each bucket (the order All/Values yield).
// Bucket order depends on the hash function and, for NewHashTab, on the
// per-table random seed, so the encoded array's order is nondeterministic
// and must not appear in fixed assertions.
//
// The elements are snapshotted under the read lock and the encoding itself
// runs without the lock, so this is safe to call concurrently with any
// table operation — and an element type with its own MarshalJSON may safely
// call back into the table (the All/Values snapshot convention).  Errors
// from the json package are returned unchanged (for example a T that cannot
// be encoded at all, such as a channel or a function).
//
// An empty table encodes as [].  A direct call on a nil table also encodes
// as [] (the "nil behaves as an empty table" read contract); note that
// json.Marshal on a nil *HashTab never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *HashTab[T]) MarshalJSON() ([]byte, error) {
	var items []T
	for v := range tt.Values() { // snapshot copied under the read lock
		items = append(items, v)
	}
	if items == nil {
		return []byte("[]"), nil // a nil or empty table marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a HashTab can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the table —
// every bucket tree is emptied and the elements are re-inserted — under one
// hold of the write lock.  The bucket count and the comparison/hash
// functions are kept, so the table stays usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the table
// lock — and a decode error (malformed JSON, a non-array document, wrong
// element types) is returned with the table untouched.  The nil/function
// guards are also checked before the lock is acquired (cmp and hash are set
// only by the constructors, so reading them unlocked is safe).
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil table or a zero-value table (no
// comparison/hash functions) panics with the standard insert-family
// message.  An empty array or null clears the table and is tolerated
// everywhere — it stores nothing.
// Complexity is O(k + n·log(n/k)) — empty k bucket trees, then n inserts —
// plus the cost of decoding the elements.
func (tt *HashTab[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("hash_tab_bt_ts: UnmarshalJSON called on a nil table")
		}
		if tt.cmp == nil || tt.hash == nil {
			panic("hash_tab_bt_ts: UnmarshalJSON called on a table with no comparison/hash functions (create the table with NewHashTab or NewHashTabFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()
	for _, b := range tt.buckets {
		b.Truncate()
	}
	tt.length = 0
	for _, item := range items {
		tt.NlInsert(item)
	}
	return nil
}
