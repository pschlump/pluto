/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map

import "encoding/json"

// MarshalJSON encodes an insertion-ordered array of key/value entries.
func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	entries := m.EntriesSlice()
	if entries == nil {
		entries = []Entry[K, V]{}
	}
	return json.Marshal(entries)
}

// UnmarshalJSON atomically replaces the map with decoded entries. Duplicate
// keys update their first position. Constructor functions are retained.
func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	var entries []Entry[K, V]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	if len(entries) != 0 {
		m.writeContract("UnmarshalJSON")
	}
	if m == nil || m.hash == nil {
		return nil
	}
	next := m.empty()
	for _, entry := range entries {
		next.Put(entry.Key, entry.Value)
	}
	*m = *next
	return nil
}
