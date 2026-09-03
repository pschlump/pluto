/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sll_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Sll can be used directly
// with the encoding/json package.  The list is encoded as a JSON array
// of its elements, head to tail (the head is element 0).
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any list operation — and an element type with its own
// MarshalJSON may safely call back into the list (the IterateOver
// snapshot convention).
//
// An empty list encodes as [].  A direct call on a nil list also
// encodes as [] (the "nil behaves as an empty list" read contract);
// note that json.Marshal on a nil *Sll never reaches this method — the
// json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (ns *Sll[T]) MarshalJSON() ([]byte, error) {
	items := ns.snapshot() // takes and releases the read lock itself
	if items == nil {
		return []byte("[]"), nil // a nil list marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Sll can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// list in order — element 0 becomes the new head — under one hold of
// the write lock.  The equality function is kept, so the list stays
// usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// list lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the list untouched.  The
// nil/equality guards are also checked before the lock is acquired (eq
// is set only by the constructors, so reading it unlocked is safe).
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil list or a zero-value list (no
// equality function) panics with the standard insert-family message.
// An empty array or null clears the list and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (ns *Sll[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 {
		if ns == nil {
			panic("sll_ts: UnmarshalJSON called on a nil list")
		}
		if ns.eq == nil {
			panic("sll_ts: UnmarshalJSON called on a list with no equality function (create the list with NewSll or NewSllFunc)")
		}
	}
	if ns == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	ns.lock.Lock()
	defer ns.lock.Unlock()
	ns.head = nil
	ns.tail = nil
	ns.length = 0
	for _, d := range items {
		x := &SllElement[T]{data: d}
		if ns.tail == nil {
			ns.head = x
		} else {
			ns.tail.next = x
		}
		ns.tail = x
		ns.length++
	}
	return nil
}
