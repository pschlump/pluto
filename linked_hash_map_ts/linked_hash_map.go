/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package linked_hash_map_ts is the thread-safe twin of linked_hash_map.
package linked_hash_map_ts

import (
	"encoding/json"
	"iter"
	"sync"

	"github.com/pschlump/pluto/linked_hash_map"
)

// Map protects an insertion-ordered map with one RWMutex.
type Map[K any, V any] struct {
	mu sync.RWMutex
	m  *linked_hash_map.Map[K, V]
}

// Entry aliases the plain map's entry type.
type Entry[K any, V any] = linked_hash_map.Entry[K, V]

func New[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{m: linked_hash_map.New[K, V]()}
}

func NewFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64) *Map[K, V] {
	return &Map[K, V]{m: linked_hash_map.NewFunc[K, V](equal, hash)}
}

func FromEntries[K comparable, V any](entries []Entry[K, V]) *Map[K, V] {
	return &Map[K, V]{m: linked_hash_map.FromEntries(entries)}
}

func (m *Map[K, V]) Get(key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.m.Get(key)
}

func (m *Map[K, V]) ContainsKey(key K) bool {
	_, found := m.Get(key)
	return found
}

func (m *Map[K, V]) Put(key K, value V) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.m.Put(key, value)
}

func (m *Map[K, V]) Delete(key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.m.Delete(key)
}

func (m *Map[K, V]) Len() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.m.Len()
}

func (m *Map[K, V]) IsEmpty() bool { return m.Len() == 0 }

func (m *Map[K, V]) Clear() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.m.Clear()
	m.mu.Unlock()
}

func entrySequence[K any, V any](entries []Entry[K, V]) iter.Seq[Entry[K, V]] {
	return func(yield func(Entry[K, V]) bool) {
		for _, entry := range entries {
			if !yield(entry) {
				return
			}
		}
	}
}

func (m *Map[K, V]) EntriesSlice() []Entry[K, V] {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.m.EntriesSlice()
}

// Entries returns an insertion-ordered snapshot with no lock held during yield.
func (m *Map[K, V]) Entries() iter.Seq[Entry[K, V]] {
	return entrySequence(m.EntriesSlice())
}

func (m *Map[K, V]) All() iter.Seq2[K, V] {
	entries := m.EntriesSlice()
	return func(yield func(K, V) bool) {
		for _, entry := range entries {
			if !yield(entry.Key, entry.Value) {
				return
			}
		}
	}
}

func (m *Map[K, V]) Keys() iter.Seq[K] {
	entries := m.EntriesSlice()
	return func(yield func(K) bool) {
		for _, entry := range entries {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

func (m *Map[K, V]) Values() iter.Seq[V] {
	entries := m.EntriesSlice()
	return func(yield func(V) bool) {
		for _, entry := range entries {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

func (m *Map[K, V]) Clone() *Map[K, V] {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &Map[K, V]{m: m.m.Clone()}
}

func (m *Map[K, V]) Equal(other *Map[K, V], equalValue func(V, V) bool) bool {
	if equalValue == nil {
		return false
	}
	left, right := m.Clone(), other.Clone()
	if left == nil {
		left = &Map[K, V]{m: &linked_hash_map.Map[K, V]{}}
	}
	if right == nil {
		right = &Map[K, V]{m: &linked_hash_map.Map[K, V]{}}
	}
	return left.m.Equal(right.m, equalValue)
}

func (m *Map[K, V]) Eq() func(K, K) bool {
	if m == nil || m.m == nil {
		return nil
	}
	return m.m.Eq()
}

func (m *Map[K, V]) Hash() func(K) uint64 {
	if m == nil || m.m == nil {
		return nil
	}
	return m.m.Hash()
}

func (m *Map[K, V]) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	m.mu.RLock()
	entries := m.m.EntriesSlice()
	m.mu.RUnlock()
	if entries == nil {
		entries = []Entry[K, V]{}
	}
	return json.Marshal(entries)
}

// UnmarshalJSON atomically replaces the map with decoded entries, keeping the
// constructor functions and index kind (a New map stays on its built-in-map
// index). The whole decode runs under the write lock.
func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m != nil && m.m.Hash() != nil {
		return m.m.UnmarshalJSON(data)
	}
	// Nil or zero-value inner map: an empty array into a nil inner map is a
	// no-op; anything else panics in NewFunc (no functions to retain).
	var entries []Entry[K, V]
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	if m.m == nil && len(entries) == 0 {
		return nil
	}
	next := linked_hash_map.NewFunc[K, V](m.m.Eq(), m.m.Hash())
	for _, entry := range entries {
		next.Put(entry.Key, entry.Value)
	}
	m.m = next
	return nil
}
