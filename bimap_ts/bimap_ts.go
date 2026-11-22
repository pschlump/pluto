/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bimap_ts is the thread-safe twin of bimap. Forward and inverse views
// share both the underlying indexes and one sync.RWMutex.
package bimap_ts

import (
	"iter"
	"sync"

	"github.com/pschlump/pluto/bimap"
)

type sharedLock struct {
	lock sync.RWMutex
}

type twinDirection[K any, V any] struct {
	plain  *bimap.HashBiMap[K, V]
	shared *sharedLock
}

// HashBiMap is a thread-safe bidirectional hash map.
type HashBiMap[K any, V any] struct {
	direction *twinDirection[K, V]
	inverse   *HashBiMap[V, K]
}

// NewHashBiMap creates an empty bimap using == and maphash.
func NewHashBiMap[K comparable, V comparable]() *HashBiMap[K, V] {
	return wrap(bimap.NewHashBiMap[K, V]())
}

// NewHashBiMapFunc creates an empty bimap with custom equality/hash functions.
func NewHashBiMapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *HashBiMap[K, V] {
	return wrap(bimap.NewHashBiMapFunc(keyEq, keyHash, valueEq, valueHash))
}

func wrap[K any, V any](plain *bimap.HashBiMap[K, V]) *HashBiMap[K, V] {
	shared := &sharedLock{}
	result := &HashBiMap[K, V]{direction: &twinDirection[K, V]{plain: plain, shared: shared}}
	inverse := &HashBiMap[V, K]{direction: &twinDirection[V, K]{plain: plain.Inverse(), shared: shared}}
	result.inverse = inverse
	inverse.inverse = result
	return result
}

// Put inserts or updates key and rejects a conflicting value without mutation.
func (m *HashBiMap[K, V]) Put(key K, value V) error {
	if m == nil || m.direction == nil {
		panic("bimap_ts: Put on a nil or zero-value HashBiMap")
	}
	m.direction.shared.lock.Lock()
	defer m.direction.shared.lock.Unlock()
	return m.direction.plain.Put(key, value)
}

// ForcePut inserts or updates key and removes conflicting associations.
func (m *HashBiMap[K, V]) ForcePut(key K, value V) {
	if m == nil || m.direction == nil {
		panic("bimap_ts: ForcePut on a nil or zero-value HashBiMap")
	}
	m.direction.shared.lock.Lock()
	defer m.direction.shared.lock.Unlock()
	m.direction.plain.ForcePut(key, value)
}

// GetValue returns the value associated with key.
func (m *HashBiMap[K, V]) GetValue(key K) (V, bool) {
	if m == nil || m.direction == nil {
		var zero V
		return zero, false
	}
	m.direction.shared.lock.RLock()
	defer m.direction.shared.lock.RUnlock()
	return m.direction.plain.GetValue(key)
}

// GetKey returns the key associated with value.
func (m *HashBiMap[K, V]) GetKey(value V) (K, bool) {
	if m == nil || m.direction == nil {
		var zero K
		return zero, false
	}
	m.direction.shared.lock.RLock()
	defer m.direction.shared.lock.RUnlock()
	return m.direction.plain.GetKey(value)
}

// DeleteKey removes key and returns its former value.
func (m *HashBiMap[K, V]) DeleteKey(key K) (V, bool) {
	if m == nil || m.direction == nil {
		var zero V
		return zero, false
	}
	m.direction.shared.lock.Lock()
	defer m.direction.shared.lock.Unlock()
	return m.direction.plain.DeleteKey(key)
}

// DeleteValue removes value and returns its former key.
func (m *HashBiMap[K, V]) DeleteValue(value V) (K, bool) {
	if m == nil || m.direction == nil {
		var zero K
		return zero, false
	}
	m.direction.shared.lock.Lock()
	defer m.direction.shared.lock.Unlock()
	return m.direction.plain.DeleteValue(value)
}

func (m *HashBiMap[K, V]) ContainsKey(key K) bool { _, found := m.GetValue(key); return found }
func (m *HashBiMap[K, V]) ContainsValue(value V) bool {
	_, found := m.GetKey(value)
	return found
}

// Len returns the number of associations.
func (m *HashBiMap[K, V]) Len() int {
	if m == nil || m.direction == nil {
		return 0
	}
	m.direction.shared.lock.RLock()
	defer m.direction.shared.lock.RUnlock()
	return m.direction.plain.Len()
}

func (m *HashBiMap[K, V]) IsEmpty() bool { return m.Len() == 0 }

// Clear atomically clears both directions and preserves existing inverse views.
func (m *HashBiMap[K, V]) Clear() {
	if m == nil || m.direction == nil {
		return
	}
	m.direction.shared.lock.Lock()
	defer m.direction.shared.lock.Unlock()
	m.direction.plain.Clear()
}

// Inverse returns the coupled view sharing the same lock and indexes.
func (m *HashBiMap[K, V]) Inverse() *HashBiMap[V, K] {
	if m == nil || m.direction == nil {
		return nil
	}
	return m.inverse
}

// EntriesSlice returns a fresh snapshot under the shared read lock.
func (m *HashBiMap[K, V]) EntriesSlice() []bimap.Association[K, V] {
	if m == nil || m.direction == nil {
		return nil
	}
	m.direction.shared.lock.RLock()
	defer m.direction.shared.lock.RUnlock()
	return m.direction.plain.EntriesSlice()
}

// Entries returns a snapshot iterator.
func (m *HashBiMap[K, V]) Entries() iter.Seq2[K, V] {
	entries := m.EntriesSlice()
	return func(yield func(K, V) bool) {
		for _, entry := range entries {
			if !yield(entry.Key, entry.Value) {
				return
			}
		}
	}
}

func (m *HashBiMap[K, V]) Keys() iter.Seq[K] {
	entries := m.EntriesSlice()
	return func(yield func(K) bool) {
		for _, entry := range entries {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

func (m *HashBiMap[K, V]) Values() iter.Seq[V] {
	entries := m.EntriesSlice()
	return func(yield func(V) bool) {
		for _, entry := range entries {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

func snapshot[K any, V any](m *HashBiMap[K, V]) *bimap.HashBiMap[K, V] {
	if m == nil || m.direction == nil {
		return nil
	}
	m.direction.shared.lock.RLock()
	defer m.direction.shared.lock.RUnlock()
	return m.direction.plain.Clone()
}

// Clone returns an independent bimap with a new lock and indexes.
func (m *HashBiMap[K, V]) Clone() *HashBiMap[K, V] {
	plain := snapshot(m)
	if plain == nil {
		return nil
	}
	return wrap(plain)
}

// Equal compares snapshots without holding two bimap locks.
func (m *HashBiMap[K, V]) Equal(other *HashBiMap[K, V]) bool {
	return snapshot(m).Equal(snapshot(other))
}
