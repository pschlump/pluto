/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a HashTab can be used directly
// with the encoding/json package.  The table is encoded as a JSON array of
// its elements, in slot order (the order Walk, All and Values use).  Slot
// order depends on the hash seed and the displacement history, so the output
// ordering is not stable across processes — decode and sort when order
// matters.
//
// The elements are snapshotted under the read lock and the encoding itself
// runs without the lock, so this is safe to call concurrently with any table
// operation — including while the background resizer is rebuilding the table
// — and an element type with its own MarshalJSON may safely call back into
// the table (the All/Values snapshot convention).
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
	tt.lock.RLock()
	items := make([]T, 0, tt.length) // non-nil, so an empty table encodes as []
	for i := range tt.slots {
		if tt.slots[i].used {
			items = append(items, tt.slots[i].data)
		}
	}
	tt.lock.RUnlock()
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a HashTab can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the table
// under one hold of the write lock.  The equality and hash functions are
// kept, so the table stays usable after unmarshaling — every decoded element
// is hashed and placed by the same rules as Insert, and a duplicate within
// data settles as the last occurrence.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the table
// lock — and a decode error (malformed JSON, a non-array document, wrong
// element types) is returned with the table untouched.  The nil/equality
// guards are also checked before the lock is acquired (eq and hash are set
// only by the constructors, so reading them unlocked is safe).
//
// The elements are decoded by the json package itself, so a T with its own
// UnmarshalJSON — or struct field tags — is honored.
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
			panic("cuckoo_ts: UnmarshalJSON called on a nil table")
		}
		if tt.eq == nil || tt.hash == nil {
			panic("cuckoo_ts: UnmarshalJSON called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()
	clear(tt.slots) // zero values of T and zero hashes, releasing references for GC
	tt.length = 0
	tt.generation++ // a full replacement, like Truncate: restart Scan walks
	for _, d := range items {
		tt.NlInsert(d) // the no-lock insert; the write lock is held
	}
	return nil
}
