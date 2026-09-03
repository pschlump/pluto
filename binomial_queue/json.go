/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binomial_queue

import "encoding/json"

// MarshalJSON implements json.Marshaler so a BinomialQueue can be used
// directly with the encoding/json package.  The queue is encoded as a
// JSON array of its elements in the queue's iteration order — the
// internal forest order of All (pre-order within each tree) — which is
// NOT sorted order; repeatedly calling DeleteMin is the way to consume
// a queue in sorted order.
//
// The elements are encoded by the json package itself, so a T with its
// own MarshalJSON — or struct field tags — is honored; only the queue
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty queue encodes as [].  A direct call on a nil queue also
// encodes as [] (the "nil behaves as an empty queue" read contract);
// note that json.Marshal on a nil *BinomialQueue never reaches this
// method — the json package writes null for nil pointers itself.
// Complexity is O(n) plus the cost of encoding the elements.
func (q *BinomialQueue[T]) MarshalJSON() ([]byte, error) {
	if q == nil {
		return []byte("[]"), nil
	}
	items := make([]T, 0, q.length)
	for _, v := range q.All() {
		items = append(items, v)
	}
	return json.Marshal(items)
}

// UnmarshalJSON implements json.Unmarshaler so a BinomialQueue can be
// used directly with the encoding/json package.  data must be a JSON
// array (or null); the decoded elements replace the current contents of
// the queue, inserted in array order.  The comparison function is kept,
// so the queue stays usable after unmarshaling.  The multiset of
// elements and the sorted drain order survive a round trip; the
// internal forest shape may differ, since it depends on insertion
// order.
//
// The elements are decoded by the json package itself, so a T with its
// own UnmarshalJSON — or struct field tags — is honored.  The decode
// runs before anything is mutated: a decode error (malformed JSON, a
// non-array document, wrong element types) is returned and leaves the
// queue untouched.
//
// Unmarshaling stores elements, so it follows the insert contract: data
// that would store an element into a nil queue or a zero-value queue
// (no comparison function) panics with the standard insert-family
// message.  An empty array or null clears the queue and is tolerated
// everywhere — it stores nothing.
// Complexity is O(n) amortized — n inserts at O(1) amortized each —
// plus the cost of decoding the elements.
func (q *BinomialQueue[T]) UnmarshalJSON(data []byte) error {
	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}

	// The insert contract only fires when an element would actually be
	// stored (the Merge precedent).
	if len(items) > 0 {
		if q == nil {
			panic("binomial_queue: UnmarshalJSON called on a nil queue")
		}
		if q.cmp == nil {
			panic("binomial_queue: UnmarshalJSON called on a queue with no comparison function (create the queue with NewBinomialQueue or NewBinomialQueueFunc)")
		}
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.Truncate()
	for _, v := range items {
		q.Insert(v)
	}
	return nil
}
