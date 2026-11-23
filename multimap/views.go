/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"iter"
	"slices"
)

// MapViewCore is the common mutation surface used by live multimap views.
// Plain and thread-safe multimaps implement it.
type MapViewCore[K any, V any] interface {
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
	EntriesSlice() []Association[K, V]
	KeySet() iter.Seq[K]
	AsMap() MapSnapshot[K, V]
}

// ValuesView is a live write-through view of one key's value collection.
// Iteration and Get return snapshots; subsequent parent changes affect later calls.
type ValuesView[K any, V any] struct {
	parent MapViewCore[K, V]
	key    K
}

func NewValuesView[K any, V any](parent MapViewCore[K, V], key K) *ValuesView[K, V] {
	return &ValuesView[K, V]{parent: parent, key: key}
}

func (v *ValuesView[K, V]) Get() ([]V, bool)      { return v.parent.Get(v.key) }
func (v *ValuesView[K, V]) Add(value V) bool      { return v.parent.Put(v.key, value) }
func (v *ValuesView[K, V]) Put(value V) bool      { return v.Add(value) }
func (v *ValuesView[K, V]) AddAll(values []V) int { return v.parent.PutAll(v.key, values) }
func (v *ValuesView[K, V]) Remove(value V) bool   { return v.parent.Remove(v.key, value) }
func (v *ValuesView[K, V]) RemoveAll() []V        { return v.parent.RemoveAll(v.key) }
func (v *ValuesView[K, V]) Clear()                { v.parent.RemoveAll(v.key) }
func (v *ValuesView[K, V]) Replace(values []V) []V {
	return v.parent.ReplaceValues(v.key, values)
}
func (v *ValuesView[K, V]) Contains(value V) bool {
	return v.parent.ContainsEntry(v.key, value)
}
func (v *ValuesView[K, V]) Len() int {
	values, _ := v.parent.Get(v.key)
	return len(values)
}
func (v *ValuesView[K, V]) IsEmpty() bool { return v.Len() == 0 }
func (v *ValuesView[K, V]) All() iter.Seq[V] {
	values, _ := v.parent.Get(v.key)
	return slices.Values(values)
}

// MapView is a live write-through map from keys to ValuesView objects.
type MapView[K any, V any] struct{ parent MapViewCore[K, V] }

func NewMapView[K any, V any](parent MapViewCore[K, V]) *MapView[K, V] {
	return &MapView[K, V]{parent: parent}
}

func (v *MapView[K, V]) Get(key K) *ValuesView[K, V] {
	return NewValuesView(v.parent, key)
}
func (v *MapView[K, V]) GetSnapshot(key K) ([]V, bool) { return v.parent.Get(key) }
func (v *MapView[K, V]) Put(key K, value V) bool       { return v.parent.Put(key, value) }
func (v *MapView[K, V]) PutAll(key K, values []V) int  { return v.parent.PutAll(key, values) }
func (v *MapView[K, V]) Remove(key K, value V) bool    { return v.parent.Remove(key, value) }
func (v *MapView[K, V]) RemoveAll(key K) []V           { return v.parent.RemoveAll(key) }
func (v *MapView[K, V]) ReplaceValues(key K, values []V) []V {
	return v.parent.ReplaceValues(key, values)
}
func (v *MapView[K, V]) ContainsKey(key K) bool            { return v.parent.ContainsKey(key) }
func (v *MapView[K, V]) Len() int                          { return v.parent.Len() }
func (v *MapView[K, V]) KeyLen() int                       { return v.parent.KeyLen() }
func (v *MapView[K, V]) IsEmpty() bool                     { return v.parent.Len() == 0 }
func (v *MapView[K, V]) Clear()                            { v.parent.Clear() }
func (v *MapView[K, V]) EntriesSlice() []Association[K, V] { return v.parent.EntriesSlice() }
func (v *MapView[K, V]) Entries() iter.Seq2[K, V] {
	entries := v.parent.EntriesSlice()
	return func(yield func(K, V) bool) {
		for _, entry := range entries {
			if !yield(entry.Key, entry.Value) {
				return
			}
		}
	}
}
func (v *MapView[K, V]) KeySet() iter.Seq[K]         { return v.parent.KeySet() }
func (v *MapView[K, V]) Snapshot() MapSnapshot[K, V] { return v.parent.AsMap() }

func (m *ListMultimap[K, V]) GetView(key K) *ValuesView[K, V] {
	return NewValuesView[K, V](&m.core, key)
}
func (m *ListMultimap[K, V]) AsMapView() *MapView[K, V] { return NewMapView[K, V](&m.core) }
func (m *SetMultimap[K, V]) GetView(key K) *ValuesView[K, V] {
	return NewValuesView[K, V](&m.core, key)
}
func (m *SetMultimap[K, V]) AsMapView() *MapView[K, V] { return NewMapView[K, V](&m.core) }
func (m *SortedSetMultimap[K, V]) GetView(key K) *ValuesView[K, V] {
	return NewValuesView[K, V](&m.core, key)
}
func (m *SortedSetMultimap[K, V]) AsMapView() *MapView[K, V] {
	return NewMapView[K, V](&m.core)
}
func (m *LinkedListMultimap[K, V]) GetView(key K) *ValuesView[K, V] {
	return NewValuesView[K, V](&m.core, key)
}
func (m *LinkedListMultimap[K, V]) AsMapView() *MapView[K, V] {
	return NewMapView[K, V](&m.core)
}
func (m *LinkedHashMultimap[K, V]) GetView(key K) *ValuesView[K, V] {
	return NewValuesView[K, V](&m.core, key)
}
func (m *LinkedHashMultimap[K, V]) AsMapView() *MapView[K, V] {
	return NewMapView[K, V](&m.core)
}
