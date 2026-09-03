/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package queue_dll

import "encoding/json"

// MarshalJSON implements json.Marshaler so a Queue can be used directly
// with the encoding/json package.  The queue is encoded as a JSON array
// of its elements, head to tail (the head — next to be dequeued — is
// element 0, matching the All iteration order).
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the queue
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
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
	items := make([]T, 0, q.length)
	for p := q.head; p != nil; p = p.next {
		items = append(items, p.data)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a Queue can be used
// directly with the encoding/json package.  data must be a JSON array
// (or null); the decoded elements replace the current contents of the
// queue in order — element 0 becomes the new head, the next element
// returned by Dequeue.  Unlike dll there is no equality function to
// keep: the zero value of Queue is usable as-is, so unmarshaling into a
// zero-value queue works.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// runs before anything is touched, so a decode error (malformed JSON, a
// non-array document, wrong element types) is returned with the queue
// untouched.
//
// Unmarshaling stores elements, so it follows the package's insert
// contract: data that would store an element into a nil queue panics
// with the standard insert-family message.  An empty array or null
// clears the queue and is tolerated everywhere — it stores nothing.
// Complexity is O(n) plus the cost of decoding the elements.
func (q *Queue[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the same precedent as Push being the only panicking call).
	if len(items) > 0 && q == nil {
		panic("queue_dll: UnmarshalJSON called on a nil queue")
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.Truncate()
	for _, d := range items {
		q.Push(d)
	}
	return nil
}
