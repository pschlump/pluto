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

// MarshalJSON encodes a snapshot as an entry array without holding the lock
// during user-defined JSON marshaling.
func (m *HashMultiset[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(snapshot(m))
}

// UnmarshalJSON decodes outside the lock, then atomically replaces the multiset.
// Negative counts and malformed JSON leave it unchanged.
func (m *HashMultiset[T]) UnmarshalJSON(data []byte) error {
	var entries []multiset.Entry[T]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	positive := false
	for _, entry := range entries {
		if entry.Count < 0 {
			return fmt.Errorf("multiset_ts: negative count for value %v", entry.Value)
		}
		positive = positive || entry.Count > 0
	}
	if m == nil {
		if positive {
			panic("multiset_ts: UnmarshalJSON called on a nil HashMultiset")
		}
		return nil
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if positive {
		m.writeContract("UnmarshalJSON")
	}
	if m.inner.Eq() == nil || m.inner.Hash() == nil {
		return nil
	}
	next := multiset.NewHashMultisetFunc(m.inner.Eq(), m.inner.Hash())
	for _, entry := range entries {
		if entry.Count > 0 {
			next.Add(entry.Value, entry.Count)
		}
	}
	m.inner = *next
	return nil
}
