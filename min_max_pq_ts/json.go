/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package min_max_pq_ts

import "encoding/json"

// MarshalJSON snapshots and encodes internal breadth-first heap order. JSON
// callbacks run without an internal queue lock.
func (q *MinMaxPriorityQueue[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(q.Slice())
}

// UnmarshalJSON decodes before taking the lock and atomically installs a
// heapified replacement, retaining the comparator and optional bound.
func (q *MinMaxPriorityQueue[T]) UnmarshalJSON(data []byte) error {
	var values []T
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if len(values) > 0 {
		q.writeContract("UnmarshalJSON")
	}
	if q == nil || q.plain == nil {
		return nil
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	next := q.plain.Clone()
	next.Clear()
	next.PushAll(values)
	q.plain = next
	return nil
}
