/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bag_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Bag can be used directly
// with the encoding/json package.  The bag is encoded as a JSON array
// of its elements in insertion order (element 0 is the first element
// added, matching the All iteration order).
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any bag operation — and an element type with its own
// MarshalJSON may safely call back into the bag (the All/Backward
// snapshot convention).
//
// An empty bag encodes as [].  A direct call on a nil bag also encodes
// as [] (the "nil behaves as an empty bag" read contract); note that
// json.Marshal on a nil *Bag never reaches this method — the json
// package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (b *Bag[T]) MarshalJSON() ([]byte, error) {
	if b == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(b.snapshot()) // an empty bag marshals as an empty array
}

// UnmarshalJSON implements json.Unmarshaler so a Bag can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// bag in order — element 0 is the first element the new All will yield
// — under one hold of the write lock.  The zero value of Bag is ready
// to use, so unmarshaling into it works without any constructor.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// bag lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the bag untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil bag panics with the standard
// insert-family message (the package's only panic, like Add).  An empty
// array or null clears the bag and is tolerated everywhere — it stores
// nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (b *Bag[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 && b == nil {
		panic("bag_ts: UnmarshalJSON called on a nil bag")
	}
	if b == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	b.lock.Lock()
	defer b.lock.Unlock()
	b.head = nil
	b.tail = nil
	b.length = 0
	for _, d := range items {
		b.noLockAdd(d)
	}
	return nil
}
