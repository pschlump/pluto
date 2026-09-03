/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package index_pq

import (
	"encoding/json"
	"fmt"
)

// jsonPair is the wire form of one (index, value) entry of the queue.
type jsonPair[T any] struct {
	K int `json:"k"`
	V T   `json:"v"`
}

// MarshalJSON implements json.Marshaler so an IndexPQ can be used
// directly with the encoding/json package.  The queue is encoded as a
// JSON array of {"k":index,"v":value} objects in priority order, minimum
// value first — the same order All yields and Pop drains.
//
// The values are encoded by the json package itself, so a T with its own
// MarshalJSON — or struct field tags — is honored; only the queue
// structure is pluto's.  Errors from the json package are returned
// unchanged (for example a T that cannot be encoded at all, such as a
// channel or a function).
//
// An empty queue encodes as [].  A direct call on a nil queue also
// encodes as [] (the "nil behaves as an empty queue" read contract);
// note that json.Marshal on a nil *IndexPQ never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) to snapshot the queue plus O(n log n) to drain the
// snapshot in priority order, plus the cost of encoding the values.
func (q *IndexPQ[T]) MarshalJSON() ([]byte, error) {
	if q == nil {
		return []byte("[]"), nil
	}
	pairs := make([]jsonPair[T], 0, q.length)
	for k, v := range q.All() { // non-destructive: drains a private snapshot
		pairs = append(pairs, jsonPair[T]{K: k, V: v})
	}
	return json.Marshal(pairs)
}

// UnmarshalJSON implements json.Unmarshaler so an IndexPQ can be used
// directly with the encoding/json package.  data must be a JSON array of
// {"k":index,"v":value} objects (or null); the decoded pairs replace the
// current contents of the queue — duplicate indices follow the Insert
// convention, so the last pair for an index wins.  The comparison
// function and the index space 0..n-1 are kept, so the queue stays
// usable after unmarshaling.
//
// The values are decoded by the json package itself, so a T with its own
// UnmarshalJSON — or struct field tags — is honored.  The whole document
// is decoded and validated before anything is stored: a decode error
// (malformed JSON, a non-array document, wrong value types) or an index
// outside the queue's index space 0..n-1 is returned and leaves the
// queue untouched.
//
// Unmarshaling stores values, so it follows the insert contract: data
// that would store a value into a nil queue or a zero-value queue (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the queue and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log n) to reinsert the pairs, plus the cost of
// decoding the values.
func (q *IndexPQ[T]) UnmarshalJSON(data []byte) error {
	var pairs []jsonPair[T]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The insert contract only fires when a value would actually be
	// stored (the Concat precedent in dll).
	if len(pairs) > 0 {
		if q == nil {
			panic("index_pq: UnmarshalJSON called on a nil queue")
		}
		if q.cmp == nil {
			panic("index_pq: UnmarshalJSON called on a queue with no comparison function (create the queue with NewIndexPQ or NewIndexPQFunc)")
		}
		// Validate the whole document before mutating anything: an
		// out-of-range index leaves the queue untouched.
		for _, p := range pairs {
			if p.K < 0 || p.K >= q.n {
				return fmt.Errorf("index_pq: UnmarshalJSON: index %d is out of the queue's index space 0..%d", p.K, q.n-1)
			}
		}
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.Truncate()
	for _, p := range pairs {
		q.Insert(p.K, p.V)
	}
	return nil
}
