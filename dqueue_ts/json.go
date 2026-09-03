/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package dqueue_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Deque can be used directly
// with the encoding/json package.  The deque is encoded as a JSON array
// of its elements, front to back (the front is element 0).
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any deque operation — and an element type with its own
// MarshalJSON may safely call back into the deque (the All/Backward
// snapshot convention).
//
// An empty deque encodes as [].  A direct call on a nil deque also
// encodes as [] (the "nil behaves as an empty deque" read contract);
// note that json.Marshal on a nil *Deque never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (q *Deque[T]) MarshalJSON() ([]byte, error) {
	items := q.snapshot() // takes and releases the read lock itself
	if items == nil {
		return []byte("[]"), nil // a nil (or empty) deque marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Deque can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// deque in order — element 0 becomes the new front — under one hold of
// the write lock.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// deque lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the deque untouched.
//
// Unmarshaling stores elements, so it follows the push contract: data
// that would store an element into a nil deque panics with the standard
// push-family message.  The zero value is a ready-to-use deque, so
// unmarshaling into one works without any constructor.  An empty array
// or null clears the deque and is tolerated everywhere — it stores
// nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (q *Deque[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The push contract only fires when an element would actually be
	// stored (the PushFront/PushBack precedent).
	if len(items) > 0 && q == nil {
		panic("dqueue_ts: UnmarshalJSON called on a nil deque")
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.lock.Lock()
	defer q.lock.Unlock()
	q.head = nil
	q.tail = nil
	q.length = 0
	for _, d := range items {
		q.noLockPushBack(d)
	}
	return nil
}
