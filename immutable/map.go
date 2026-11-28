/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"fmt"
	"iter"
)

// Entry is one immutable-map key/value association.
type Entry[K any, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value"`
}

// Map is an immutable hash map with first-entry iteration order.
type Map[K any, V any] struct {
	entries []Entry[K, V]
	index   *hashIndex[K]
}

func NewMap[K comparable, V any](entries []Entry[K, V]) (*Map[K, V], error) {
	return newMap(newNaturalIndex[K](len(entries)), entries)
}

// NewMapFunc rejects duplicate keys and defensively copies entries.
func NewMapFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64, entries []Entry[K, V]) (*Map[K, V], error) {
	return newMap(newHashIndexSized(equal, hash, len(entries)), entries)
}

func newMap[K any, V any](index *hashIndex[K], entries []Entry[K, V]) (*Map[K, V], error) {
	result := &Map[K, V]{index: index, entries: append([]Entry[K, V](nil), entries...)}
	for position, entry := range result.entries {
		if !index.addUnique(entry.Key, position) {
			return nil, fmt.Errorf("%w at entry %d", ErrDuplicateKey, position)
		}
	}
	return result, nil
}

func CopyOfMap[K any, V any](value *Map[K, V]) *Map[K, V] { return value }

func (m *Map[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

func (m *Map[K, V]) IsEmpty() bool { return m.Len() == 0 }

// Get returns key's value. Complexity is expected O(1).
func (m *Map[K, V]) Get(key K) (value V, found bool) {
	if m == nil {
		return value, false
	}
	position, found := m.index.get(key)
	if found {
		value = m.entries[position].Value
	}
	return value, found
}

func (m *Map[K, V]) ContainsKey(key K) bool {
	_, found := m.Get(key)
	return found
}

func (m *Map[K, V]) Entries() iter.Seq[Entry[K, V]] {
	var values []Entry[K, V]
	if m != nil {
		values = m.entries
	}
	return sequence(values)
}

func (m *Map[K, V]) EntriesSlice() []Entry[K, V] {
	if m == nil {
		return nil
	}
	return append([]Entry[K, V]{}, m.entries...)
}

func (m *Map[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

func (m *Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

// Equal reports key/value content equality independent of iteration order.
func (m *Map[K, V]) Equal(other *Map[K, V]) bool {
	if m.Len() != other.Len() {
		return false
	}
	for entry := range m.Entries() {
		value, found := other.Get(entry.Key)
		if !found || any(value) != any(entry.Value) {
			return false
		}
	}
	return true
}

// MapBuilder incrementally constructs independent Map snapshots.
type MapBuilder[K any, V any] struct {
	entries []Entry[K, V]
	equal   func(K, K) bool
	hash    func(K) uint64
	natural func(hint int) *hashIndex[K] // set by NewMapBuilder only
}

func NewMapBuilder[K comparable, V any]() *MapBuilder[K, V] {
	equal, hash := naturalFunctions[K]()
	builder := NewMapBuilderFunc[K, V](equal, hash)
	builder.natural = newNaturalIndex[K]
	return builder
}

func NewMapBuilderFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64) *MapBuilder[K, V] {
	if equal == nil || hash == nil {
		panic("immutable: NewMapBuilderFunc requires non-nil functions")
	}
	return &MapBuilder[K, V]{equal: equal, hash: hash}
}

func (b *MapBuilder[K, V]) Put(key K, value V) {
	b.entries = append(b.entries, Entry[K, V]{Key: key, Value: value})
}

func (b *MapBuilder[K, V]) PutAll(entries []Entry[K, V]) {
	b.entries = append(b.entries, entries...)
}

func (b *MapBuilder[K, V]) Build() (*Map[K, V], error) {
	if b == nil {
		return nil, nil
	}
	if b.natural != nil {
		return newMap(b.natural(len(b.entries)), b.entries)
	}
	return NewMapFunc(b.equal, b.hash, b.entries)
}
