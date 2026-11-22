/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap

import "encoding/json"

// MarshalJSON encodes associations as an entry array in unspecified hash order.
func (m *HashBiMap[K, V]) MarshalJSON() ([]byte, error) {
	entries := m.EntriesSlice()
	if entries == nil {
		entries = []Association[K, V]{}
	}
	return json.Marshal(entries)
}

// UnmarshalJSON decodes outside the map, then replaces its contents while
// retaining equality/hash functions. Malformed input and value conflicts leave
// the map unchanged. A nil or zero-value map accepts only an empty array.
func (m *HashBiMap[K, V]) UnmarshalJSON(data []byte) error {
	var entries []Association[K, V]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	if len(entries) > 0 {
		m.writeContract("UnmarshalJSON")
	}
	if m == nil || m.direction == nil {
		return nil
	}
	next := NewHashBiMapFunc(m.direction.keyEq, m.direction.keyHash, m.direction.valueEq, m.direction.valueHash)
	for _, entry := range entries {
		if err := next.Put(entry.Key, entry.Value); err != nil {
			return err
		}
	}
	m.Clear()
	for _, entry := range next.EntriesSlice() {
		m.ForcePut(entry.Key, entry.Value)
	}
	return nil
}
