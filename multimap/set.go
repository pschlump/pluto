/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"iter"

	"github.com/pschlump/pluto/multiset"
)

// SetMultimap stores duplicate-free value sets. Value iteration order is
// unspecified.
type SetMultimap[K any, V any] struct {
	core core[K, V, *valueSet[V]]
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

// valueSet is one key's SetMultimap collection: the package table with an
// empty payload. Adding an equal value stores the latest representative
// (hash_grow.Insert's replace semantics, which the set variant always had).
type valueSet[V any] = table[V, struct{}]

func newSetMultimap[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueEq func(V, V) bool, valueHash func(V) uint64) *SetMultimap[K, V] {
	newValues := func() *valueSet[V] {
		return newTable[V, struct{}](valueEq, valueHash)
	}
	add := func(values *valueSet[V], value V) (*valueSet[V], bool) {
		if values == nil {
			values = newValues()
		}
		index, h, found := values.find(value)
		if found {
			values.slots[index].key = value
			return values, false
		}
		values.insertAt(index, h, value, struct{}{})
		return values, true
	}
	return &SetMultimap[K, V]{core: newCore(keyEq, keyHash, collectionOps[V, *valueSet[V]]{
		unique:        true,
		newCollection: newValues,
		add:           add,
		remove: func(values *valueSet[V], value V) (*valueSet[V], bool) {
			return values, values.remove(value)
		},
		contains: func(values *valueSet[V], value V) bool {
			return values.lookup(value) != nil
		},
		length: func(values *valueSet[V]) int {
			if values == nil {
				return 0
			}
			return values.length
		},
		values: func(values *valueSet[V]) []V {
			return values.keys()
		},
		replace: func(values []V) *valueSet[V] {
			result := newValues()
			for _, value := range values {
				add(result, value)
			}
			return result
		},
	}, valueEq)}
}

// KeyEq, KeyHash, and ValueEq expose the stored functions for snapshots.
func (m *SetMultimap[K, V]) KeyEq() func(K, K) bool {
	if m == nil {
		return nil
	}
	return m.core.eq
}
func (m *SetMultimap[K, V]) KeyHash() func(K) uint64 {
	if m == nil {
		return nil
	}
	return m.core.hash
}
func (m *SetMultimap[K, V]) ValueEq() func(V, V) bool {
	if m == nil {
		return nil
	}
	return m.core.valueEq
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

// ValueCount returns how many values key has without copying them: the
// allocation-free replacement for len(Get(key)), and the same work as Guava's
// get(key).size(). Values are distinct. An absent key, a nil multimap,
// and the zero value report 0.
// Complexity is expected O(1).
func (m *SetMultimap[K, V]) ValueCount(key K) int {
	if m == nil {
		return 0
	}
	return m.core.ValueCount(key)
}

// ValuesOf iterates key's values in the same order as Get without copying them: the
// allocation-free alternative to Get for callers that only read the values.
// Get must copy because it returns a slice the caller owns. Do not modify the
// multimap while iterating. The lookup runs when iteration starts, inside the
// returned function, which keeps ValuesOf small enough to inline into a range
// loop. An absent key, a nil multimap, and the zero value yield nothing.
// Complexity is expected O(1) lookup plus O(values for key) to iterate.
func (m *SetMultimap[K, V]) ValuesOf(key K) iter.Seq[V] {
	return func(yield func(V) bool) {
		if m == nil {
			return
		}
		collection := m.core.collection(key)
		if collection == nil || *collection == nil {
			return
		}
		// Slot order, exactly as Get's values op (table.keys) produces it.
		slots := (*collection).slots
		for i := range slots {
			if slots[i].hash != 0 && !yield(slots[i].key) {
				return
			}
		}
	}
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
