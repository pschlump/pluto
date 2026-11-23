/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package evicting_queue

import "encoding/json"

// MarshalJSON encodes elements as an array from oldest to newest.
func (q *Queue[T]) MarshalJSON() ([]byte, error) {
	values := q.Slice()
	if values == nil {
		values = []T{}
	}
	return json.Marshal(values)
}

// UnmarshalJSON replaces the queue with the newest values that fit its fixed
// capacity. Decode errors leave the queue unchanged.
func (q *Queue[T]) UnmarshalJSON(data []byte) error {
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
	q.Clear()
	q.PushAll(values)
	return nil
}
