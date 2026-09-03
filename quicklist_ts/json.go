/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package quicklist_ts

import "encoding/json"

// MarshalJSON implements json.Marshaler so a QuickList can be used
// directly with the encoding/json package.  The list is encoded as a
// JSON array of its elements, head to tail (the head is element 0) —
// the same order All iterates.
//
// The elements are snapshotted under the write lock — the walk
// materializes compressed segments, exactly as At reads them — and the
// encoding itself runs without the lock, so this is safe to call
// concurrently with any list operation (the All/Backward snapshot
// convention).  The elements are encoded by the json package itself, so
// a T with its own MarshalJSON — or struct field tags — is honored;
// only the list structure is pluto's.  Errors from the json package are
// returned unchanged (for example a T that cannot be encoded at all,
// such as a channel or a function).
//
// An empty list encodes as [].  A direct call on a nil list also
// encodes as [] (the "nil behaves as an empty list" read contract);
// note that json.Marshal on a nil *QuickList never reaches this method
// — the json package writes null for nil pointers itself.
// Complexity is O(S + n) plus the cost of encoding the elements.
func (q *QuickList[T]) MarshalJSON() ([]byte, error) {
	items := q.snapshot() // takes and releases the write lock itself
	if items == nil {
		return []byte("[]"), nil // a nil list marshals as an empty array
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a QuickList can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// list in order — element 0 becomes the new head — under one hold of
// the write lock.  The configuration (fill target, byte cap,
// compression codec and depth) is kept, so the list — including a
// zero-value one, which needs no constructor — stays usable after
// unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// element-level UnmarshalJSON methods, which must not run under the
// list lock — and a decode error (malformed JSON, a non-array document,
// wrong element types) is returned with the list untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil list panics with the standard
// insert-family message.  An empty array or null clears the list and is
// tolerated everywhere — it stores nothing.
// Complexity is O(n) amortized-O(1) tail pushes plus the cost of
// decoding the elements.
func (q *QuickList[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 && q == nil {
		panic("quicklist_ts: UnmarshalJSON called on a nil QuickList")
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.lock.Lock()
	defer q.lock.Unlock()
	q.NlDeleteRange(0, -1)
	for _, v := range items {
		q.NlPushTail(v)
	}
	return nil
}
