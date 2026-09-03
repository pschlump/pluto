/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sharded_hash_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a ShardedHash can be used
// directly with the encoding/json package.  The table is encoded as a JSON
// array of its elements, in the stripe-then-bucket order All and Values
// use; like that order, the array order depends on the hash seed and the
// per-stripe growth history and varies from process to process — never
// assert a fixed order.
//
// The elements are snapshotted stripe by stripe, each under its own read
// lock, and the encoding itself runs without any lock held, so this is
// safe to call concurrently with any table operation — and an element
// type with its own MarshalJSON may safely call back into the table (the
// All/Values snapshot convention; per stripe the snapshot is
// point-in-time, stripes are not mutually consistent — there is no global
// lock to make them so).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the table
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty table encodes as [].  A direct call on a nil table also
// encodes as [] (the "nil behaves as an empty table" read contract);
// note that json.Marshal on a nil *ShardedHash never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *ShardedHash[T]) MarshalJSON() ([]byte, error) {
	if tt == nil {
		return []byte("[]"), nil
	}
	var snap []T
	for _, s := range tt.stripes {
		s.lock.RLock()
		for _, head := range s.tab.heads {
			for n := head; n != nil; n = n.next {
				snap = append(snap, n.data)
			}
		}
		s.lock.RUnlock()
	}
	if snap == nil {
		return []byte("[]"), nil // an empty (or zero-value) table marshals as an empty array
	}
	return json.Marshal(snap)
}

// UnmarshalJSON implements json.Unmarshaler so a ShardedHash can be used
// directly with the encoding/json package.  data must be a JSON array (or
// null); the decoded elements replace the current contents of the table.
// The equality and hash functions are kept, so the table stays usable
// after unmarshaling.
//
// data is decoded before any lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under a stripe
// lock — and a decode error (malformed JSON, a non-array document, wrong
// element types) is returned with the table untouched.  The nil/function
// guards are also checked before any lock is acquired (eq and hash are
// set only by the constructors, so reading them unlocked is safe).
//
// The replacement runs as a Truncate followed by per-element inserts:
// stripes are locked one at a time, never two at once (the package's
// locking rule), so the replacement as a whole is not atomic — a
// concurrent reader can observe a partially refilled table.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil table or a zero-value table (no
// equality/hash functions) panics with the standard insert-family
// message.  An empty array or null clears the table and is tolerated
// everywhere — it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (tt *ShardedHash[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("sharded_hash_ts: UnmarshalJSON called on a nil table")
		}
		if tt.eq == nil || tt.hash == nil {
			panic("sharded_hash_ts: UnmarshalJSON called on a table with no equality/hash functions (create the table with NewShardedHash or NewShardedHashFunc)")
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
