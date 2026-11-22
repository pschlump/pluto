/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap_ts

import (
	"encoding/json"

	"github.com/pschlump/pluto/bimap"
)

// MarshalJSON encodes a snapshot without holding the lock during user JSON code.
func (m *HashBiMap[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(snapshot(m))
}

// UnmarshalJSON decodes before locking, then atomically replaces both indexes.
// Malformed input and value conflicts leave the original bimap unchanged.
func (m *HashBiMap[K, V]) UnmarshalJSON(data []byte) error {
	var entries []bimap.Association[K, V]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	if m == nil || m.direction == nil {
		if len(entries) > 0 {
			panic("bimap_ts: UnmarshalJSON on a nil or zero-value HashBiMap")
		}
		return nil
	}
	m.direction.shared.lock.Lock()
	defer m.direction.shared.lock.Unlock()
	next := m.direction.plain.Clone()
	next.Clear()
	for _, entry := range entries {
		if err := next.Put(entry.Key, entry.Value); err != nil {
			return err
		}
	}
	m.direction.plain.Clear()
	for _, entry := range next.EntriesSlice() {
		m.direction.plain.ForcePut(entry.Key, entry.Value)
	}
	return nil
}
