/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package skip_list_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a SkipList can be used
// directly with the encoding/json package.  The list is encoded as a
// JSON array of its elements in ascending order (the level-0 chain —
// element 0 is the smallest).
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any list operation — and an element type with its own
// MarshalJSON may safely call back into the list (the All/Backward
// snapshot convention).
//
// An empty list encodes as [].  A direct call on a nil list also
// encodes as [] (the "nil behaves as an empty list" read contract);
// note that json.Marshal on a nil *SkipList never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (tt *SkipList[T]) MarshalJSON() ([]byte, error) {
	items := tt.toSlice() // takes and releases the read lock itself
	if items == nil {
		return []byte("[]"), nil // a nil or empty list marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a SkipList can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// list under one hold of the write lock — the result is the set of
// decoded elements, ordered by the list's comparison function, with
// duplicate-decoding elements replacing earlier ones exactly as Insert
// does.  The comparison function is kept, so the list stays usable
// after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// list lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the list untouched.  The
// nil/comparison guards are also checked before the lock is acquired
// (cmp is set only by the constructors, so reading it unlocked is
// safe).
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil list or a zero-value list (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the list and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log n) expected — n elements, each inserted in
// O(log n) expected — plus the cost of decoding the elements.
func (tt *SkipList[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if tt == nil {
			panic("skip_list_ts: UnmarshalJSON called on a nil list")
		}
		if tt.cmp == nil {
			panic("skip_list_ts: UnmarshalJSON called on a list with no comparison function (create the list with NewSkipList or NewSkipListFunc)")
		}
	}
	if tt == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	tt.lock.Lock()
	defer tt.lock.Unlock()
	tt.head = nil
	tt.level = 0
	tt.length = 0
	for _, item := range items {
		tt.insertLocked(item)
	}
	return nil
}
