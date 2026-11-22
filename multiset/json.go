/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON encodes entries as a JSON array. Hash entry order is unspecified;
// tree entry order is ascending. Empty values encode as [].
func (m *HashMultiset[T]) MarshalJSON() ([]byte, error) {
	entries := m.EntriesSlice()
	if entries == nil {
		entries = []Entry[T]{}
	}
	return json.Marshal(entries)
}

// UnmarshalJSON replaces m with decoded entries, retaining equality and hash
// functions. Duplicate values are added. Malformed JSON and negative counts
// return an error without changing m; count overflow panics.
func (m *HashMultiset[T]) UnmarshalJSON(data []byte) error {
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
	if m == nil || m.tab == nil {
		return nil
	}
	next := newHashMultiset(m.eq, m.hash)
	for _, entry := range entries {
		if entry.Count > 0 {
			next.Add(entry.Value, entry.Count)
		}
	}
	*m = *next
	return nil
}

// MarshalJSON encodes ascending entries as a JSON array.
func (m *TreeMultiset[T]) MarshalJSON() ([]byte, error) {
	entries := m.EntriesSlice()
	if entries == nil {
		entries = []Entry[T]{}
	}
	return json.Marshal(entries)
}

// UnmarshalJSON replaces m with decoded entries, retaining its comparator.
// Malformed JSON and negative counts leave m unchanged.
func (m *TreeMultiset[T]) UnmarshalJSON(data []byte) error {
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
	if m == nil || m.cmp == nil {
		return nil
	}
	next := NewTreeMultisetFunc(m.cmp)
	for _, entry := range entries {
		if entry.Count > 0 {
			next.Add(entry.Value, entry.Count)
		}
	}
	*m = *next
	return nil
}

func hasPositive[T any](entries []Entry[T]) bool {
	for _, entry := range entries {
		if entry.Count > 0 {
			return true
		}
	}
	return false
}
