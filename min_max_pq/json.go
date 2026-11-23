/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq

import "encoding/json"

// MarshalJSON encodes elements in internal breadth-first heap order.
func (q *MinMaxPriorityQueue[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.Slice())
}

// UnmarshalJSON decodes and heapifies a complete replacement before modifying
// the queue. The comparator and optional bound are retained.
func (q *MinMaxPriorityQueue[T]) UnmarshalJSON(data []byte) error {
	var values []T
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) > 0 {
		q.writeContract("UnmarshalJSON")
	}
	if q == nil {
		return nil
	}
	next := &MinMaxPriorityQueue[T]{compare: q.compare, maximumSize: q.maximumSize}
	next.PushAll(values)
	*q = *next
	return nil
}
