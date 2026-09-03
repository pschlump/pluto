/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package queue_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Queue can be used directly
// with the encoding/json package.  The queue is encoded as a JSON array
// of its elements, head (next to be dequeued) to tail (the head is
// element 0).
//
// The elements are snapshotted under the read lock and the encoding
// itself runs without the lock, so this is safe to call concurrently
// with any queue operation — and an element type with its own
// MarshalJSON may safely call back into the queue (the All/Backward
// snapshot convention).
//
// An empty queue encodes as [].  A direct call on a nil queue also
// encodes as [] (the "nil behaves as an empty queue" read contract);
// note that json.Marshal on a nil *Queue never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (q *Queue[T]) MarshalJSON() ([]byte, error) {
	if q == nil {
		return []byte("[]"), nil
	}
	q.lock.RLock()
	snapshot := make([]T, len(q.data))
	copy(snapshot, q.data)
	q.lock.RUnlock()
	return json.Marshal(snapshot) // an empty queue marshals as an empty array
}

// UnmarshalJSON implements json.Unmarshaler so a Queue can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// queue in order — element 0 becomes the new head — under one hold of
// the write lock.  The zero value of Queue is ready to use, so
// unmarshaling into it works without any constructor.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// queue lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the queue untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil queue panics with the standard
// insert-family message (the package's only panic, like Push/Enqueue).
// An empty array or null clears the queue and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (q *Queue[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 && q == nil {
		panic("queue_ts: UnmarshalJSON called on a nil queue")
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.lock.Lock()
	defer q.lock.Unlock()
	q.data = items
	return nil
}
