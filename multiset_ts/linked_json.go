/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset_ts

import (
	"encoding/json"
	"fmt"

	"github.com/pschlump/pluto/multiset"
)

func (m *LinkedHashMultiset[T]) MarshalJSON() ([]byte, error) {
	entries := m.EntriesSlice()
	if entries == nil {
		entries = []multiset.Entry[T]{}
	}
	return json.Marshal(entries)
}

func (m *LinkedHashMultiset[T]) UnmarshalJSON(data []byte) error {
	var entries []multiset.Entry[T]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Count < 0 {
			return fmt.Errorf("multiset_ts: negative count for value %v", entry.Value)
		}
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.inner.Eq() == nil && len(entries) == 0 {
		return nil
	}
	next := multiset.NewLinkedHashMultisetFunc(m.inner.Eq(), m.inner.Hash())
	for _, entry := range entries {
		if entry.Count > 0 {
			next.Add(entry.Value, entry.Count)
		}
	}
	m.inner = *next
	return nil
}
