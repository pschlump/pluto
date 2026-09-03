/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package index_pq_ts

import (
	"encoding/json"
	"fmt"
)

// jsonPair is the wire form of one (index, value) pair of the queue.
type jsonPair[T any] struct {
	Index int `json:"index"`
	Value T   `json:"value"`
}

// MarshalJSON implements json.Marshaler so an IndexPQ can be used
// directly with the encoding/json package.  The queue is encoded as a
// JSON array of {"index":k,"value":v} pair objects in priority order —
// the minimum value first, exactly the order All iterates.
//
// The pairs are snapshotted under the read lock and the encoding itself
// runs without the lock, so this is safe to call concurrently with any
// queue operation — and a value type with its own MarshalJSON may safely
// call back into the queue (the All snapshot convention).  Errors from
// the json package are returned unchanged (for example a T that cannot
// be encoded at all, such as a channel or a function).
//
// An empty queue encodes as [].  A direct call on a nil queue also
// encodes as [] (the "nil behaves as an empty queue" read contract);
// note that json.Marshal on a nil *IndexPQ never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n log n) — the snapshot drains in priority order —
// plus the cost of encoding the values.
func (q *IndexPQ[T]) MarshalJSON() ([]byte, error) {
	pairs := q.snapshotPairs() // takes and releases the read lock itself
	if pairs == nil {
		return []byte("[]"), nil // a nil or empty queue marshals as an empty array
	}
	return json.Marshal(pairs)
}

// snapshotPairs drains a private copy of the queue into a slice of
// (index, value) pairs in priority order, minimum value first.  It takes
// and releases the read lock itself; draining the copy runs lock-free.
// A nil or empty queue returns nil.
func (q *IndexPQ[T]) snapshotPairs() []jsonPair[T] {
	if q == nil {
		return nil
	}
	q.lock.RLock()
	snapshot := q.clone()
	q.lock.RUnlock()
	var pairs []jsonPair[T]
	for snapshot.length > 0 {
		k, v := snapshot.deletePos(0)
		pairs = append(pairs, jsonPair[T]{Index: k, Value: v})
	}
	return pairs
}

// UnmarshalJSON implements json.Unmarshaler so an IndexPQ can be used
// directly with the encoding/json package.  data must be a JSON array of
// {"index":k,"value":v} pair objects (or null); the decoded pairs
// replace the current contents of the queue under one hold of the write
// lock.  Insertion is in document order with the Insert semantics — a
// repeated index keeps the last value — and the comparison function is
// kept, so the queue stays usable after unmarshaling.
//
// data is decoded before the lock is taken — the json package runs
// value-level UnmarshalJSON methods, which must not run under the queue
// lock — and a decode error (malformed JSON, a non-array document, wrong
// value types) is returned with the queue untouched.  An index outside
// the queue's index space 0..n-1 is likewise rejected with an error
// before anything is mutated.  The nil/comparison guards are also
// checked before the lock is acquired (cmp and n are set only by the
// constructors, so reading them unlocked is safe).
//
// Unmarshaling stores values, so it follows the insert contract: data
// that would store a value into a nil queue or a zero-value queue (no
// comparison function) panics with the standard insert-family message.
// An empty array or null clears the queue and is tolerated everywhere —
// it stores nothing.
// Complexity is O(n log n) plus the cost of decoding the values.
func (q *IndexPQ[T]) UnmarshalJSON(data []byte) error {
	var pairs []jsonPair[T]
	if err := json.Unmarshal(data, &pairs); err != nil {
		return err
	}

	// The insert contract only fires when a value would actually be
	// stored (the same precedent as Insert itself).
	if len(pairs) > 0 {
		if q == nil {
			panic("index_pq_ts: UnmarshalJSON called on a nil queue")
		}
		if q.cmp == nil {
			panic("index_pq_ts: UnmarshalJSON called on a queue with no comparison function (create the queue with NewIndexPQ or NewIndexPQFunc)")
		}
		for _, p := range pairs {
			if p.Index < 0 || p.Index >= q.n {
				return fmt.Errorf("index_pq_ts: UnmarshalJSON: index %d is out of the queue's index space 0..%d", p.Index, q.n-1)
			}
		}
	}
	if q == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	q.lock.Lock()
	defer q.lock.Unlock()
	q.length = 0
	q.pq = q.pq[:0]
	for i := range q.qp {
		q.qp[i] = -1
	}
	clear(q.vals) // zero the value slots so the GC can reclaim them
	for _, p := range pairs {
		q.nlInsert(p.Index, p.Value)
	}
	return nil
}
