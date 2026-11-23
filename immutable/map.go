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
	equal, hash := naturalFunctions[K]()
	return NewMapFunc(equal, hash, entries)
}

// NewMapFunc rejects duplicate keys and defensively copies entries.
func NewMapFunc[K any, V any](equal func(K, K) bool, hash func(K) uint64, entries []Entry[K, V]) (*Map[K, V], error) {
	result := &Map[K, V]{index: newHashIndex(equal, hash), entries: append([]Entry[K, V](nil), entries...)}
	for position, entry := range result.entries {
		if !result.index.add(entry.Key, position) {
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

func (m *Map[K, V]) Get(key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	position, found := m.index.get(key)
	if !found {
		var zero V
		return zero, false
	}
	return m.entries[position].Value, true
}

func (m *Map[K, V]) ContainsKey(key K) bool {
	_, found := m.Get(key)
	return found
}

func (m *Map[K, V]) Entries() iter.Seq[Entry[K, V]] {
	if m == nil {
		return sequence[Entry[K, V]](nil)
	}
	return sequence(m.entries)
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
}

func NewMapBuilder[K comparable, V any]() *MapBuilder[K, V] {
	equal, hash := naturalFunctions[K]()
	return NewMapBuilderFunc[K, V](equal, hash)
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
	return NewMapFunc(b.equal, b.hash, b.entries)
}
