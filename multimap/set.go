/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"iter"

	"github.com/pschlump/pluto/hash_grow"
	"github.com/pschlump/pluto/multiset"
)

// SetMultimap stores duplicate-free value sets. Value iteration order is
// unspecified.
type SetMultimap[K any, V any] struct {
	core core[K, V, *hash_grow.HashTab[V]]
}

// NewSetMultimap creates an empty set multimap using == for keys and values.
func NewSetMultimap[K comparable, V comparable]() *SetMultimap[K, V] {
	keyEq, keyHash := naturalFunctions[K]()
	valueEq, valueHash := naturalFunctions[V]()
	return newSetMultimap(keyEq, keyHash, valueEq, valueHash)
}

// NewSetMultimapFunc creates a set multimap with custom key and value functions.
func NewSetMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *SetMultimap[K, V] {
	if valueEq == nil || valueHash == nil {
		panic("multimap: NewSetMultimapFunc requires non-nil value equality and hash functions")
	}
	return newSetMultimap(keyEq, keyHash, valueEq, valueHash)
}

func newSetMultimap[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *SetMultimap[K, V] {
	newValues := func() *hash_grow.HashTab[V] {
		return hash_grow.NewHashTabFunc(valueEq, valueHash, 8, 0)
	}
	return &SetMultimap[K, V]{core: newCore(keyEq, keyHash, collectionOps[V, *hash_grow.HashTab[V]]{
		unique:        true,
		newCollection: newValues,
		add: func(values *hash_grow.HashTab[V], value V) (*hash_grow.HashTab[V], bool) {
			if values == nil {
				values = newValues()
			}
			return values, values.Insert(value)
		},
		remove: func(values *hash_grow.HashTab[V], value V) (*hash_grow.HashTab[V], bool) {
			if values == nil {
				return nil, false
			}
			return values, values.Delete(value)
		},
		contains: func(values *hash_grow.HashTab[V], value V) bool {
			return values != nil && values.Contains(value)
		},
		length: func(values *hash_grow.HashTab[V]) int {
			if values == nil {
				return 0
			}
			return values.Len()
		},
		values: func(values *hash_grow.HashTab[V]) []V {
			if values == nil {
				return nil
			}
			return values.Slice()
		},
		replace: func(values []V) *hash_grow.HashTab[V] {
			result := newValues()
			for _, value := range values {
				result.Insert(value)
			}
			return result
		},
	})}
}

// Put adds a unique association and reports whether it was new.
func (m *SetMultimap[K, V]) Put(key K, value V) bool      { return m.core.Put(key, value) }
func (m *SetMultimap[K, V]) PutAll(key K, values []V) int { return m.core.PutAll(key, values) }
func (m *SetMultimap[K, V]) Get(key K) ([]V, bool)        { return m.core.Get(key) }
func (m *SetMultimap[K, V]) Remove(key K, value V) bool   { return m.core.Remove(key, value) }
func (m *SetMultimap[K, V]) RemoveAll(key K) []V          { return m.core.RemoveAll(key) }
func (m *SetMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	return m.core.ReplaceValues(key, values)
}
func (m *SetMultimap[K, V]) ContainsKey(key K) bool { return m.core.ContainsKey(key) }
func (m *SetMultimap[K, V]) ContainsEntry(key K, value V) bool {
	return m.core.ContainsEntry(key, value)
}
func (m *SetMultimap[K, V]) ContainsValue(value V) bool { return m.core.ContainsValue(value) }
func (m *SetMultimap[K, V]) Len() int                   { return m.core.Len() }
func (m *SetMultimap[K, V]) KeyLen() int                { return m.core.KeyLen() }
func (m *SetMultimap[K, V]) IsEmpty() bool              { return m.core.IsEmpty() }
func (m *SetMultimap[K, V]) Clear()                     { m.core.Clear() }
func (m *SetMultimap[K, V]) Entries() iter.Seq2[K, V]   { return m.core.Entries() }
func (m *SetMultimap[K, V]) Keys() iter.Seq[K]          { return m.core.Keys() }
func (m *SetMultimap[K, V]) Values() iter.Seq[V]        { return m.core.Values() }
func (m *SetMultimap[K, V]) KeySet() iter.Seq[K]        { return m.core.KeySet() }
func (m *SetMultimap[K, V]) EntriesSlice() []Association[K, V] {
	return m.core.EntriesSlice()
}
func (m *SetMultimap[K, V]) KeyMultiset() *multiset.HashMultiset[K] {
	return m.core.KeyMultiset()
}
func (m *SetMultimap[K, V]) AsMap() MapSnapshot[K, V] { return m.core.AsMap() }
