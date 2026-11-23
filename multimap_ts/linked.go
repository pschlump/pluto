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

type plainLinked[K any, V any] interface {
	Put(K, V) bool
	PutAll(K, []V) int
	Get(K) ([]V, bool)
	Remove(K, V) bool
	RemoveAll(K) []V
	ReplaceValues(K, []V) []V
	ContainsKey(K) bool
	ContainsEntry(K, V) bool
	ContainsValue(V) bool
	Len() int
	KeyLen() int
	Clear()
	EntriesSlice() []multimap.Association[K, V]
	KeySet() iter.Seq[K]
	KeyMultiset() *multiset.LinkedHashMultiset[K]
	AsMap() multimap.MapSnapshot[K, V]
}

type linkedTwin[K any, V any] struct {
	lock  sync.RWMutex
	inner plainLinked[K, V]
}

func (m *linkedTwin[K, V]) Put(key K, value V) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Put(key, value)
}
func (m *linkedTwin[K, V]) PutAll(key K, values []V) int {
	if len(values) == 0 {
		return 0
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.PutAll(key, values)
}
func (m *linkedTwin[K, V]) Get(key K) ([]V, bool) {
	if m == nil || m.inner == nil {
		return nil, false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Get(key)
}
func (m *linkedTwin[K, V]) Remove(key K, value V) bool {
	if m == nil || m.inner == nil {
		return false
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Remove(key, value)
}
func (m *linkedTwin[K, V]) RemoveAll(key K) []V {
	if m == nil || m.inner == nil {
		return nil
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.RemoveAll(key)
}
func (m *linkedTwin[K, V]) ReplaceValues(key K, values []V) []V {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.ReplaceValues(key, values)
}
func (m *linkedTwin[K, V]) ContainsKey(key K) bool {
	if m == nil || m.inner == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsKey(key)
}
func (m *linkedTwin[K, V]) ContainsEntry(key K, value V) bool {
	if m == nil || m.inner == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsEntry(key, value)
}
func (m *linkedTwin[K, V]) ContainsValue(value V) bool {
	if m == nil || m.inner == nil {
		return false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.ContainsValue(value)
}
func (m *linkedTwin[K, V]) Len() int {
	if m == nil || m.inner == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Len()
}
func (m *linkedTwin[K, V]) KeyLen() int {
	if m == nil || m.inner == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.KeyLen()
}
func (m *linkedTwin[K, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *linkedTwin[K, V]) Clear() {
	if m == nil || m.inner == nil {
		return
	}
	m.lock.Lock()
	m.inner.Clear()
	m.lock.Unlock()
}
func (m *linkedTwin[K, V]) EntriesSlice() []multimap.Association[K, V] {
	if m == nil || m.inner == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.EntriesSlice()
}
func (m *linkedTwin[K, V]) Entries() iter.Seq2[K, V] {
	return snapshotAssociations(m.EntriesSlice())
}
func (m *linkedTwin[K, V]) Keys() iter.Seq[K]   { return snapshotKeys(m.EntriesSlice()) }
func (m *linkedTwin[K, V]) Values() iter.Seq[V] { return snapshotValues(m.EntriesSlice()) }
func (m *linkedTwin[K, V]) KeySet() iter.Seq[K] {
	if m == nil || m.inner == nil {
		return slices.Values([]K(nil))
	}
	m.lock.RLock()
	keys := []K{}
	for key := range m.inner.KeySet() {
		keys = append(keys, key)
	}
	m.lock.RUnlock()
	return slices.Values(keys)
}
func (m *linkedTwin[K, V]) KeyMultiset() *multiset.LinkedHashMultiset[K] {
	if m == nil || m.inner == nil {
		return &multiset.LinkedHashMultiset[K]{}
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.KeyMultiset()
}
func (m *linkedTwin[K, V]) AsMap() multimap.MapSnapshot[K, V] {
	if m == nil || m.inner == nil {
		return multimap.MapSnapshot[K, V]{}
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.AsMap()
}

// LinkedListMultimap is the thread-safe insertion-ordered list multimap.
type LinkedListMultimap[K any, V any] struct{ linkedTwin[K, V] }

func NewLinkedListMultimap[K comparable, V comparable]() *LinkedListMultimap[K, V] {
	return &LinkedListMultimap[K, V]{linkedTwin: linkedTwin[K, V]{inner: multimap.NewLinkedListMultimap[K, V]()}}
}
func NewLinkedListMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool) *LinkedListMultimap[K, V] {
	return &LinkedListMultimap[K, V]{linkedTwin: linkedTwin[K, V]{inner: multimap.NewLinkedListMultimapFunc(keyEq, keyHash, valueEq)}}
}

// LinkedHashMultimap is the thread-safe insertion-ordered set multimap.
type LinkedHashMultimap[K any, V any] struct{ linkedTwin[K, V] }

func NewLinkedHashMultimap[K comparable, V comparable]() *LinkedHashMultimap[K, V] {
	return &LinkedHashMultimap[K, V]{linkedTwin: linkedTwin[K, V]{inner: multimap.NewLinkedHashMultimap[K, V]()}}
}
func NewLinkedHashMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *LinkedHashMultimap[K, V] {
	return &LinkedHashMultimap[K, V]{linkedTwin: linkedTwin[K, V]{inner: multimap.NewLinkedHashMultimapFunc(keyEq, keyHash, valueEq, valueHash)}}
}
