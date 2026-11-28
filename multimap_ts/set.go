/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap_ts

import (
	"iter"
	"slices"
	"sync"

	"github.com/pschlump/pluto/multimap"
	"github.com/pschlump/pluto/multiset"
)

// SetMultimap is a thread-safe duplicate-free set multimap.
type SetMultimap[K any, V any] struct {
	inner multimap.SetMultimap[K, V]
	lock  sync.RWMutex
}

func NewSetMultimap[K comparable, V comparable]() *SetMultimap[K, V] {
	return &SetMultimap[K, V]{inner: *multimap.NewSetMultimap[K, V]()}
}

func NewSetMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *SetMultimap[K, V] {
	return &SetMultimap[K, V]{inner: *multimap.NewSetMultimapFunc(keyEq, keyHash, valueEq, valueHash)}
}

func (m *SetMultimap[K, V]) Put(key K, value V) bool {
	if m == nil {
		panic("multimap_ts: Put on a nil SetMultimap")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Put(key, value)
}
func (m *SetMultimap[K, V]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	if m == nil {
		panic("multimap_ts: PutAll on a nil SetMultimap")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.PutAll(key, values)
}
func (m *SetMultimap[K, V]) Get(key K) ([]V, bool) {
	if m == nil {
		return nil, false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Get(key)
}
func (m *SetMultimap[K, V]) Remove(key K, value V) bool {
	if m == nil {
		return false
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Remove(key, value)
}
func (m *SetMultimap[K, V]) RemoveAll(key K) []V {
	if m == nil {
		return nil
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.RemoveAll(key)
}
func (m *SetMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	if m == nil {
		panic("multimap_ts: ReplaceValues on a nil SetMultimap")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.ReplaceValues(key, values)
}

// ValueCount returns how many values key has, under the read lock, without
// copying them. An absent key, a nil multimap, and the zero value report 0.
// Complexity is expected O(1).
func (m *SetMultimap[K, V]) ValueCount(key K) int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ValueCount(key)
}

// ValuesOf iterates a snapshot of key's values, in the plain type's order,
// taken under the read lock when ValuesOf is called. The snapshot keeps the
// iteration safe while other goroutines write, so unlike the plain type's
// ValuesOf it copies the values once.
// Complexity is expected O(1) lookup plus O(values for key).
func (m *SetMultimap[K, V]) ValuesOf(key K) iter.Seq[V] {
	values, _ := m.Get(key)
	return slices.Values(values)
}

func (m *SetMultimap[K, V]) ContainsKey(key K) bool {
	if m == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsKey(key)
}
func (m *SetMultimap[K, V]) ContainsEntry(key K, value V) bool {
	if m == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsEntry(key, value)
}
func (m *SetMultimap[K, V]) ContainsValue(value V) bool {
	if m == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsValue(value)
}
func (m *SetMultimap[K, V]) Len() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Len()
}
func (m *SetMultimap[K, V]) KeyLen() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.KeyLen()
}
func (m *SetMultimap[K, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *SetMultimap[K, V]) Clear() {
	if m == nil {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.inner.Clear()
}
func (m *SetMultimap[K, V]) Entries() iter.Seq2[K, V] { return snapshotAssociations(m.EntriesSlice()) }
func (m *SetMultimap[K, V]) EntriesSlice() []multimap.Association[K, V] {
	if m == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.EntriesSlice()
}
func (m *SetMultimap[K, V]) Keys() iter.Seq[K]   { return snapshotKeys(m.EntriesSlice()) }
func (m *SetMultimap[K, V]) Values() iter.Seq[V] { return snapshotValues(m.EntriesSlice()) }
func (m *SetMultimap[K, V]) KeySet() iter.Seq[K] {
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
func (m *SetMultimap[K, V]) KeyMultiset() *multiset.HashMultiset[K] {
	if m == nil {
		return &multiset.HashMultiset[K]{}
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.KeyMultiset()
}
func (m *SetMultimap[K, V]) AsMap() multimap.MapSnapshot[K, V] {
	if m == nil {
		return multimap.MapSnapshot[K, V]{}
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.AsMap()
}
