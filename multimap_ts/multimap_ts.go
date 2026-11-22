/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package multimap_ts provides mutex-guarded twins of Pluto's multimaps.
// Compound mutations are atomic and every iterator is a snapshot.
package multimap_ts

import (
	"iter"
	"slices"
	"sync"

	"github.com/pschlump/pluto/multimap"
	"github.com/pschlump/pluto/multiset"
)

// ListMultimap is a thread-safe list multimap.
type ListMultimap[K any, V any] struct {
	inner multimap.ListMultimap[K, V]
	lock  sync.RWMutex
}

func NewListMultimap[K comparable, V comparable]() *ListMultimap[K, V] {
	return &ListMultimap[K, V]{inner: *multimap.NewListMultimap[K, V]()}
}

func NewListMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool) *ListMultimap[K, V] {
	return &ListMultimap[K, V]{inner: *multimap.NewListMultimapFunc(keyEq, keyHash, valueEq)}
}

func (m *ListMultimap[K, V]) Put(key K, value V) bool {
	if m == nil {
		panic("multimap_ts: Put on a nil ListMultimap")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Put(key, value)
}
func (m *ListMultimap[K, V]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	if m == nil {
		panic("multimap_ts: PutAll on a nil ListMultimap")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.PutAll(key, values)
}
func (m *ListMultimap[K, V]) Get(key K) ([]V, bool) {
	if m == nil {
		return nil, false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Get(key)
}
func (m *ListMultimap[K, V]) Remove(key K, value V) bool {
	if m == nil {
		return false
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Remove(key, value)
}
func (m *ListMultimap[K, V]) RemoveAll(key K) []V {
	if m == nil {
		return nil
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.RemoveAll(key)
}
func (m *ListMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	if m == nil {
		panic("multimap_ts: ReplaceValues on a nil ListMultimap")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.ReplaceValues(key, values)
}
func (m *ListMultimap[K, V]) ContainsKey(key K) bool {
	if m == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsKey(key)
}
func (m *ListMultimap[K, V]) ContainsEntry(key K, value V) bool {
	if m == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsEntry(key, value)
}
func (m *ListMultimap[K, V]) ContainsValue(value V) bool {
	if m == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsValue(value)
}
func (m *ListMultimap[K, V]) Len() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Len()
}
func (m *ListMultimap[K, V]) KeyLen() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.KeyLen()
}
func (m *ListMultimap[K, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *ListMultimap[K, V]) Clear() {
	if m == nil {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Clear()
}
func (m *ListMultimap[K, V]) Entries() iter.Seq2[K, V] {
	return snapshotAssociations(m.EntriesSlice())
}
func (m *ListMultimap[K, V]) EntriesSlice() []multimap.Association[K, V] {
	if m == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.EntriesSlice()
}
func (m *ListMultimap[K, V]) Keys() iter.Seq[K] {
	return snapshotKeys(m.EntriesSlice())
}
func (m *ListMultimap[K, V]) Values() iter.Seq[V] {
	return snapshotValues(m.EntriesSlice())
}
func (m *ListMultimap[K, V]) KeySet() iter.Seq[K] {
	if m == nil {
		return slices.Values([]K(nil))
	}
	m.lock.RLock()
	keys := make([]K, 0, m.inner.KeyLen())
	for key := range m.inner.KeySet() {
		keys = append(keys, key)
	}
	m.lock.RUnlock()
	return slices.Values(keys)
}
func (m *ListMultimap[K, V]) KeyMultiset() *multiset.HashMultiset[K] {
	if m == nil {
		return &multiset.HashMultiset[K]{}
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.KeyMultiset()
}
func (m *ListMultimap[K, V]) AsMap() multimap.MapSnapshot[K, V] {
	if m == nil {
		return multimap.MapSnapshot[K, V]{}
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.AsMap()
}

func snapshotAssociations[K any, V any](entries []multimap.Association[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, entry := range entries {
			if !yield(entry.Key, entry.Value) {
				return
			}
		}
	}
}

func snapshotKeys[K any, V any](entries []multimap.Association[K, V]) iter.Seq[K] {
	return func(yield func(K) bool) {
		for _, entry := range entries {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

func snapshotValues[K any, V any](entries []multimap.Association[K, V]) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, entry := range entries {
			if !yield(entry.Value) {
				return
			}
		}
	}
}
