/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"hash/maphash"
	"iter"
	"slices"

	"github.com/pschlump/pluto/multiset"
)

// ListMultimap stores ordered value lists with duplicate values permitted.
type ListMultimap[K any, V any] struct {
	core core[K, V, []V]
}

// NewListMultimap creates an empty list multimap using == for keys and values.
func NewListMultimap[K comparable, V comparable]() *ListMultimap[K, V] {
	keyEq, keyHash := naturalFunctions[K]()
	return newListMultimap(keyEq, keyHash, func(a, b V) bool { return a == b })
}

// NewListMultimapFunc creates a list multimap with custom key equality/hash
// and value equality. Value equality is used by Remove and ContainsEntry.
func NewListMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool) *ListMultimap[K, V] {
	if valueEq == nil {
		panic("multimap: NewListMultimapFunc requires a non-nil value equality function")
	}
	return newListMultimap(keyEq, keyHash, valueEq)
}

func newListMultimap[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool) *ListMultimap[K, V] {
	return &ListMultimap[K, V]{core: newCore(keyEq, keyHash, collectionOps[V, []V]{
		newCollection: func() []V { return nil },
		add: func(values []V, value V) ([]V, bool) {
			return append(values, value), true
		},
		remove: func(values []V, value V) ([]V, bool) {
			for index, candidate := range values {
				if valueEq(candidate, value) {
					copy(values[index:], values[index+1:])
					var zero V
					values[len(values)-1] = zero
					return values[:len(values)-1], true
				}
			}
			return values, false
		},
		contains: func(values []V, value V) bool {
			return slices.ContainsFunc(values, func(candidate V) bool { return valueEq(candidate, value) })
		},
		length:  func(values []V) int { return len(values) },
		values:  func(values []V) []V { return slices.Clone(values) },
		replace: func(values []V) []V { return slices.Clone(values) },
	}, valueEq)}
}

func naturalFunctions[T comparable]() (func(T, T) bool, func(T) uint64) {
	seed := maphash.MakeSeed()
	return func(a, b T) bool { return a == b }, func(value T) uint64 {
		return maphash.Comparable(seed, value)
	}
}

// KeyEq, KeyHash, and ValueEq expose the stored functions for snapshots.
func (m *ListMultimap[K, V]) KeyEq() func(K, K) bool {
	if m == nil {
		return nil
	}
	return m.core.eq
}
func (m *ListMultimap[K, V]) KeyHash() func(K) uint64 {
	if m == nil {
		return nil
	}
	return m.core.hash
}
func (m *ListMultimap[K, V]) ValueEq() func(V, V) bool {
	if m == nil {
		return nil
	}
	return m.core.valueEq
}

// Put appends value to key's list and always returns true.
func (m *ListMultimap[K, V]) Put(key K, value V) bool { return m.core.Put(key, value) }

// PutAll appends values and returns the number added.
func (m *ListMultimap[K, V]) PutAll(key K, values []V) int { return m.core.PutAll(key, values) }

// Get returns a snapshot of key's values in insertion order.
func (m *ListMultimap[K, V]) Get(key K) ([]V, bool) { return m.core.Get(key) }

// Remove deletes the first equal value for key.
func (m *ListMultimap[K, V]) Remove(key K, value V) bool { return m.core.Remove(key, value) }

// RemoveAll removes key and returns its former values in insertion order.
func (m *ListMultimap[K, V]) RemoveAll(key K) []V { return m.core.RemoveAll(key) }

// ReplaceValues replaces key's list and returns its former values.
func (m *ListMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	return m.core.ReplaceValues(key, values)
}

func (m *ListMultimap[K, V]) ContainsKey(key K) bool { return m.core.ContainsKey(key) }
func (m *ListMultimap[K, V]) ContainsEntry(key K, value V) bool {
	return m.core.ContainsEntry(key, value)
}
func (m *ListMultimap[K, V]) ContainsValue(value V) bool { return m.core.ContainsValue(value) }
func (m *ListMultimap[K, V]) Len() int                   { return m.core.Len() }
func (m *ListMultimap[K, V]) KeyLen() int                { return m.core.KeyLen() }
func (m *ListMultimap[K, V]) IsEmpty() bool              { return m.core.IsEmpty() }
func (m *ListMultimap[K, V]) Clear()                     { m.core.Clear() }
func (m *ListMultimap[K, V]) Entries() iter.Seq2[K, V]   { return m.core.Entries() }
func (m *ListMultimap[K, V]) Keys() iter.Seq[K]          { return m.core.Keys() }
func (m *ListMultimap[K, V]) Values() iter.Seq[V]        { return m.core.Values() }
func (m *ListMultimap[K, V]) KeySet() iter.Seq[K]        { return m.core.KeySet() }
func (m *ListMultimap[K, V]) EntriesSlice() []Association[K, V] {
	return m.core.EntriesSlice()
}
func (m *ListMultimap[K, V]) KeyMultiset() *multiset.HashMultiset[K] {
	return m.core.KeyMultiset()
}
func (m *ListMultimap[K, V]) AsMap() MapSnapshot[K, V] { return m.core.AsMap() }
