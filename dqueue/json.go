/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package dqueue

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Deque can be used directly
// with the encoding/json package.  The deque is encoded as a JSON array
// of its elements, front to back (the front is element 0 — the same
// order All iterates in).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the deque
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty deque encodes as [].  A direct call on a nil deque also
// encodes as [] (the "nil behaves as an empty deque" read contract);
// note that json.Marshal on a nil *Deque never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (q *Deque[T]) MarshalJSON() ([]byte, error) {
	if q == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, q.length)
	for p := q.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Deque can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// deque in order — element 0 becomes the new front.  The deque needs no
// constructor, so a zero-value Deque is ready to receive elements.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// happens before anything is mutated, so a decode error (malformed
// JSON, a non-array document, wrong element types) is returned and
// leaves the deque untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil deque panics, like PushFront
// and PushBack do.  An empty array or null clears the deque and is
// tolerated everywhere — it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (q *Deque[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored.
	if len(items) > 0 && q == nil {
		panic("dqueue: UnmarshalJSON called on a nil deque")
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.Truncate()
	for _, d := range items {
		q.PushBack(d)
	}
	return nil
}
