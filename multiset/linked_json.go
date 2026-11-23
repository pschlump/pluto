/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON encodes entries in first-insertion order.
func (m *LinkedHashMultiset[T]) MarshalJSON() ([]byte, error) {
	entries := m.EntriesSlice()
	if entries == nil {
		entries = []Entry[T]{}
	}
	return json.Marshal(entries)
}

// UnmarshalJSON atomically replaces entries while retaining equality and hash.
func (m *LinkedHashMultiset[T]) UnmarshalJSON(data []byte) error {
	var entries []Entry[T]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Count < 0 {
			return fmt.Errorf("multiset: negative count for value %v", entry.Value)
		}
	}
	if hasPositive(entries) {
		m.writeContract("UnmarshalJSON")
	}
	if m == nil || m.counts == nil {
		return nil
	}
	next := NewLinkedHashMultisetFunc(m.Eq(), m.Hash())
	for _, entry := range entries {
		if entry.Count > 0 {
			next.Add(entry.Value, entry.Count)
		}
	}
	*m = *next
	return nil
}
