/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"cmp"
	"iter"
	"slices"
	"sort"

	"github.com/pschlump/pluto/multiset"
)

// SortedSetMultimap stores duplicate-free value sets in comparator order.
// Keys remain hash-ordered; only each key's values are sorted.
type SortedSetMultimap[K any, V any] struct {
	core core[K, V, []V]
}

// NewSortedSetMultimap creates an empty multimap with natural key equality and
// value ordering.
func NewSortedSetMultimap[K comparable, V cmp.Ordered]() *SortedSetMultimap[K, V] {
	keyEq, keyHash := naturalFunctions[K]()
	return NewSortedSetMultimapFunc(keyEq, keyHash, compareValues[V])
}

// NewSortedSetMultimapFunc creates an empty multimap with custom key functions
// and value comparator.
func NewSortedSetMultimapFunc[K any, V any](keyEq func(K, K) bool, keyHash func(K) uint64, valueCompare func(V, V) int) *SortedSetMultimap[K, V] {
	if valueCompare == nil {
		panic("multimap: NewSortedSetMultimapFunc requires a non-nil value comparator")
	}
	return &SortedSetMultimap[K, V]{core: newCore(keyEq, keyHash, collectionOps[V, []V]{
		unique:        true,
		newCollection: func() []V { return nil },
		add: func(values []V, value V) ([]V, bool) {
			index := sort.Search(len(values), func(index int) bool {
				return valueCompare(values[index], value) >= 0
			})
			if index < len(values) && valueCompare(values[index], value) == 0 {
				return values, false
			}
			var zero V
			values = append(values, zero)
			copy(values[index+1:], values[index:])
			values[index] = value
			return values, true
		},
		remove: func(values []V, value V) ([]V, bool) {
			index := sort.Search(len(values), func(index int) bool {
				return valueCompare(values[index], value) >= 0
			})
			if index == len(values) || valueCompare(values[index], value) != 0 {
				return values, false
			}
			copy(values[index:], values[index+1:])
			var zero V
			values[len(values)-1] = zero
			return values[:len(values)-1], true
		},
		contains: func(values []V, value V) bool {
			index := sort.Search(len(values), func(index int) bool {
				return valueCompare(values[index], value) >= 0
			})
			return index < len(values) && valueCompare(values[index], value) == 0
		},
		length: func(values []V) int { return len(values) },
		values: func(values []V) []V { return slices.Clone(values) },
		replace: func(values []V) []V {
			result := slices.Clone(values)
			sort.SliceStable(result, func(i, j int) bool { return valueCompare(result[i], result[j]) < 0 })
			if len(result) == 0 {
				return result
			}
			write := 1
			for read := 1; read < len(result); read++ {
				if valueCompare(result[write-1], result[read]) != 0 {
					result[write] = result[read]
					write++
				}
			}
			var zero V
			for index := write; index < len(result); index++ {
				result[index] = zero
			}
			return result[:write]
		},
	}, func(a, b V) bool { return valueCompare(a, b) == 0 })}
}

func compareValues[V cmp.Ordered](a, b V) int {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	default:
		return 0
	}
}

// KeyEq, KeyHash, and ValueEq expose the stored functions for snapshots.
func (m *SortedSetMultimap[K, V]) KeyEq() func(K, K) bool {
	if m == nil {
		return nil
	}
	return m.core.eq
}
func (m *SortedSetMultimap[K, V]) KeyHash() func(K) uint64 {
	if m == nil {
		return nil
	}
	return m.core.hash
}
func (m *SortedSetMultimap[K, V]) ValueEq() func(V, V) bool {
	if m == nil {
		return nil
	}
	return m.core.valueEq
}

// Put adds a unique association and reports whether it was new.
func (m *SortedSetMultimap[K, V]) Put(key K, value V) bool { return m.core.Put(key, value) }
func (m *SortedSetMultimap[K, V]) PutAll(key K, values []V) int {
	return m.core.PutAll(key, values)
}
func (m *SortedSetMultimap[K, V]) Get(key K) ([]V, bool)      { return m.core.Get(key) }
func (m *SortedSetMultimap[K, V]) Remove(key K, value V) bool { return m.core.Remove(key, value) }
func (m *SortedSetMultimap[K, V]) RemoveAll(key K) []V        { return m.core.RemoveAll(key) }
func (m *SortedSetMultimap[K, V]) ReplaceValues(key K, values []V) []V {
	return m.core.ReplaceValues(key, values)
}
func (m *SortedSetMultimap[K, V]) ContainsKey(key K) bool { return m.core.ContainsKey(key) }
func (m *SortedSetMultimap[K, V]) ContainsEntry(key K, value V) bool {
	return m.core.ContainsEntry(key, value)
}
func (m *SortedSetMultimap[K, V]) ContainsValue(value V) bool { return m.core.ContainsValue(value) }
func (m *SortedSetMultimap[K, V]) Len() int                   { return m.core.Len() }
func (m *SortedSetMultimap[K, V]) KeyLen() int                { return m.core.KeyLen() }
func (m *SortedSetMultimap[K, V]) IsEmpty() bool              { return m.core.IsEmpty() }
func (m *SortedSetMultimap[K, V]) Clear()                     { m.core.Clear() }
func (m *SortedSetMultimap[K, V]) Entries() iter.Seq2[K, V]   { return m.core.Entries() }
func (m *SortedSetMultimap[K, V]) Keys() iter.Seq[K]          { return m.core.Keys() }
func (m *SortedSetMultimap[K, V]) Values() iter.Seq[V]        { return m.core.Values() }
func (m *SortedSetMultimap[K, V]) KeySet() iter.Seq[K]        { return m.core.KeySet() }
func (m *SortedSetMultimap[K, V]) EntriesSlice() []Association[K, V] {
	return m.core.EntriesSlice()
}
func (m *SortedSetMultimap[K, V]) KeyMultiset() *multiset.HashMultiset[K] {
	return m.core.KeyMultiset()
}
func (m *SortedSetMultimap[K, V]) AsMap() MapSnapshot[K, V] { return m.core.AsMap() }
