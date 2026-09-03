/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package priority_queue

import "encoding/json"

// MarshalJSON implements json.Marshaler so a PriorityQueue can be used
// directly with the encoding/json package.  The queue is encoded as a
// JSON array of its elements in priority order, minimum element first —
// the order All iterates in.  The encoding is non-destructive: it walks
// the same snapshot All builds, so the queue is unchanged afterwards.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the queue
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty queue encodes as [].  A direct call on a nil or zero-value
// queue also encodes as [] (the "nil behaves as an empty queue" read
// contract); note that json.Marshal on a nil *PriorityQueue never
// reaches this method — the json package writes null for nil pointers
// itself.
// Complexity is O(n log n) to build the ordered snapshot, plus the cost
// of encoding the elements.
func (pq *PriorityQueue[T]) MarshalJSON() ([]byte, error) {
	if pq == nil || pq.h == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, pq.h.Len())
	for v := range pq.All() {
		items = append(items, v)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a PriorityQueue can be
// used directly with the encoding/json package.  data must be a JSON
// array (or null); the decoded elements replace the current contents of
// the queue.  The array order does not matter to the queue — elements
// are inserted one by one and come back out in priority order — but the
// comparison function is kept, so the queue stays usable after
// unmarshaling.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  A decode
// error (malformed JSON, a non-array document, wrong element types) is
// returned and leaves the queue untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil queue or a zero-value queue
// (no underlying heap) panics with the standard insert-family message.
// An empty array or null clears the queue and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log n) to insert the elements, plus the cost of
// decoding them.
func (pq *PriorityQueue[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Truncate precedent: clearing stores nothing).
	if len(items) > 0 {
		if pq == nil || pq.h == nil {
			panic("priority_queue: UnmarshalJSON called on a nil or zero-value queue (create the queue with NewPriorityQueue or NewPriorityQueueFunc)")
		}
	}
	if pq == nil || pq.h == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	pq.Truncate()
	for _, d := range items {
		pq.Insert(d)
	}
	return nil
}
