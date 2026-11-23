/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"iter"

	"github.com/pschlump/pluto/multiset"
)

// LinkedListMultimap preserves first-key insertion order and global association order.
type LinkedListMultimap[K any, V any] struct {
	core linkedCore[K, V]
}

func NewLinkedListMultimap[K comparable, V comparable]() *LinkedListMultimap[K, V] {
	keyEq, keyHash := naturalFunctions[K]()
	return NewLinkedListMultimapFunc(keyEq, keyHash, func(a, b V) bool { return a == b })
}

func NewLinkedListMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool) *LinkedListMultimap[K, V] {
	return &LinkedListMultimap[K, V]{core: newLinkedCore(keyEq, keyHash, valueEq, nil, false)}
}

// LinkedHashMultimap preserves first-key and global unique-association order.
type LinkedHashMultimap[K any, V any] struct {
	core linkedCore[K, V]
}

func NewLinkedHashMultimap[K comparable, V comparable]() *LinkedHashMultimap[K, V] {
	keyEq, keyHash := naturalFunctions[K]()
	valueEq, valueHash := naturalFunctions[V]()
	return NewLinkedHashMultimapFunc(keyEq, keyHash, valueEq, valueHash)
}

func NewLinkedHashMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *LinkedHashMultimap[K, V] {
	return &LinkedHashMultimap[K, V]{core: newLinkedCore(keyEq, keyHash, valueEq, valueHash, true)}
}

func (m *LinkedListMultimap[K, V]) KeyEq() func(K, K) bool       { return m.core.eq }
func (m *LinkedListMultimap[K, V]) KeyHash() func(K) uint64      { return m.core.hash }
func (m *LinkedListMultimap[K, V]) ValueEq() func(V, V) bool     { return m.core.valueEq }
func (m *LinkedListMultimap[K, V]) Put(key K, value V) bool      { return m.core.Put(key, value) }
func (m *LinkedListMultimap[K, V]) PutAll(key K, values []V) int { return m.core.PutAll(key, values) }
func (m *LinkedListMultimap[K, V]) Get(key K) ([]V, bool)        { return m.core.Get(key) }
func (m *LinkedListMultimap[K, V]) Remove(key K, value V) bool   { return m.core.Remove(key, value) }
func (m *LinkedListMultimap[K, V]) RemoveAll(key K) []V          { return m.core.RemoveAll(key) }
func (m *LinkedListMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	return m.core.ReplaceValues(key, values)
}
func (m *LinkedListMultimap[K, V]) ContainsKey(key K) bool { return m.core.ContainsKey(key) }
func (m *LinkedListMultimap[K, V]) ContainsEntry(key K, value V) bool {
	return m.core.ContainsEntry(key, value)
}
func (m *LinkedListMultimap[K, V]) ContainsValue(value V) bool { return m.core.ContainsValue(value) }
func (m *LinkedListMultimap[K, V]) Len() int                   { return m.core.Len() }
func (m *LinkedListMultimap[K, V]) KeyLen() int                { return m.core.KeyLen() }
func (m *LinkedListMultimap[K, V]) IsEmpty() bool              { return m.core.IsEmpty() }
func (m *LinkedListMultimap[K, V]) Clear()                     { m.core.Clear() }
func (m *LinkedListMultimap[K, V]) Entries() iter.Seq2[K, V]   { return m.core.Entries() }
func (m *LinkedListMultimap[K, V]) Keys() iter.Seq[K]          { return m.core.Keys() }
func (m *LinkedListMultimap[K, V]) Values() iter.Seq[V]        { return m.core.Values() }
func (m *LinkedListMultimap[K, V]) KeySet() iter.Seq[K]        { return m.core.KeySet() }
func (m *LinkedListMultimap[K, V]) EntriesSlice() []Association[K, V] {
	return m.core.EntriesSlice()
}
func (m *LinkedListMultimap[K, V]) KeyMultiset() *multiset.LinkedHashMultiset[K] {
	result := multiset.NewLinkedHashMultisetFunc(m.core.eq, m.core.hash)
	for key := range m.core.KeySet() {
		values, _ := m.core.Get(key)
		result.Add(key, len(values))
	}
	return result
}
func (m *LinkedListMultimap[K, V]) AsMap() MapSnapshot[K, V] { return m.core.AsMap() }

func (m *LinkedHashMultimap[K, V]) KeyEq() func(K, K) bool   { return m.core.eq }
func (m *LinkedHashMultimap[K, V]) KeyHash() func(K) uint64  { return m.core.hash }
func (m *LinkedHashMultimap[K, V]) ValueEq() func(V, V) bool { return m.core.valueEq }
func (m *LinkedHashMultimap[K, V]) Put(key K, value V) bool  { return m.core.Put(key, value) }
func (m *LinkedHashMultimap[K, V]) PutAll(key K, values []V) int {
	return m.core.PutAll(key, values)
}
func (m *LinkedHashMultimap[K, V]) Get(key K) ([]V, bool)      { return m.core.Get(key) }
func (m *LinkedHashMultimap[K, V]) Remove(key K, value V) bool { return m.core.Remove(key, value) }
func (m *LinkedHashMultimap[K, V]) RemoveAll(key K) []V        { return m.core.RemoveAll(key) }
func (m *LinkedHashMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	return m.core.ReplaceValues(key, values)
}
func (m *LinkedHashMultimap[K, V]) ContainsKey(key K) bool { return m.core.ContainsKey(key) }
func (m *LinkedHashMultimap[K, V]) ContainsEntry(key K, value V) bool {
	return m.core.ContainsEntry(key, value)
}
func (m *LinkedHashMultimap[K, V]) ContainsValue(value V) bool { return m.core.ContainsValue(value) }
func (m *LinkedHashMultimap[K, V]) Len() int                   { return m.core.Len() }
func (m *LinkedHashMultimap[K, V]) KeyLen() int                { return m.core.KeyLen() }
func (m *LinkedHashMultimap[K, V]) IsEmpty() bool              { return m.core.IsEmpty() }
func (m *LinkedHashMultimap[K, V]) Clear()                     { m.core.Clear() }
func (m *LinkedHashMultimap[K, V]) Entries() iter.Seq2[K, V]   { return m.core.Entries() }
func (m *LinkedHashMultimap[K, V]) Keys() iter.Seq[K]          { return m.core.Keys() }
func (m *LinkedHashMultimap[K, V]) Values() iter.Seq[V]        { return m.core.Values() }
func (m *LinkedHashMultimap[K, V]) KeySet() iter.Seq[K]        { return m.core.KeySet() }
func (m *LinkedHashMultimap[K, V]) EntriesSlice() []Association[K, V] {
	return m.core.EntriesSlice()
}
func (m *LinkedHashMultimap[K, V]) KeyMultiset() *multiset.LinkedHashMultiset[K] {
	result := multiset.NewLinkedHashMultisetFunc(m.core.eq, m.core.hash)
	for key := range m.core.KeySet() {
		values, _ := m.core.Get(key)
		result.Add(key, len(values))
	}
	return result
}
func (m *LinkedHashMultimap[K, V]) AsMap() MapSnapshot[K, V] { return m.core.AsMap() }
