/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue_ts

import "encoding/json"

// MarshalJSON snapshots the queue and encodes it oldest to newest. JSON
// callbacks run without an internal queue lock.
func (q *Queue[T]) MarshalJSON() ([]byte, error) {
	values := q.Slice()
	if values == nil {
		values = []T{}
	}
	return json.Marshal(values)
}

// UnmarshalJSON decodes before taking the lock, then atomically replaces the
// queue with the newest values that fit its fixed capacity.
func (q *Queue[T]) UnmarshalJSON(data []byte) error {
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
	q.plain.Clear()
	q.plain.PushAll(values)
	return nil
}
