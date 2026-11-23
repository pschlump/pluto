/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"fmt"
	"iter"

	"github.com/pschlump/pluto/bimap"
)

// BiMap is an immutable bidirectional map. Its inverse shares the indexes and
// preserves the original construction order.
type BiMap[K any, V any] struct {
	entries []bimap.Association[K, V]
	forward *hashIndex[K]
	reverse *hashIndex[V]
	inverse *BiMap[V, K]
}

func NewBiMap[K comparable, V comparable](entries []bimap.Association[K, V]) (*BiMap[K, V], error) {
	keyEqual, keyHash := naturalFunctions[K]()
	valueEqual, valueHash := naturalFunctions[V]()
	return NewBiMapFunc(keyEqual, keyHash, valueEqual, valueHash, entries)
}

func NewBiMapFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool, valueHash func(V) uint64, entries []bimap.Association[K, V]) (*BiMap[K, V], error) {
	result := &BiMap[K, V]{
		entries: append([]bimap.Association[K, V](nil), entries...),
		forward: newHashIndex(keyEqual, keyHash), reverse: newHashIndex(valueEqual, valueHash),
	}
	for position, entry := range result.entries {
		if !result.forward.add(entry.Key, position) {
			return nil, fmt.Errorf("%w at entry %d", ErrDuplicateKey, position)
		}
		if !result.reverse.add(entry.Value, position) {
			return nil, fmt.Errorf("%w at entry %d", ErrDuplicateValue, position)
		}
	}
	inverseEntries := make([]bimap.Association[V, K], len(result.entries))
	for index, entry := range result.entries {
		inverseEntries[index] = bimap.Association[V, K]{Key: entry.Value, Value: entry.Key}
	}
	inverse := &BiMap[V, K]{entries: inverseEntries, forward: result.reverse, reverse: result.forward}
	result.inverse = inverse
	inverse.inverse = result
	return result, nil
}

func CopyOfBiMap[K any, V any](value *BiMap[K, V]) *BiMap[K, V] { return value }

func FreezeHashBiMap[K any, V any](source *bimap.HashBiMap[K, V]) *BiMap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		result := &BiMap[K, V]{}
		result.inverse = &BiMap[V, K]{}
		result.inverse.inverse = result
		return result
	}
	result, err := NewBiMapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.ValueHash(), source.EntriesSlice())
	if err != nil {
		panic("immutable: valid HashBiMap contains a duplicate: " + err.Error())
	}
	return result
}

// Inverse returns the coupled immutable inverse. Inverse().Inverse() is the
// original value.
func (m *BiMap[K, V]) Inverse() *BiMap[V, K] {
	if m == nil {
		return nil
	}
	return m.inverse
}

func (m *BiMap[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

func (m *BiMap[K, V]) IsEmpty() bool { return m.Len() == 0 }

func (m *BiMap[K, V]) GetValue(key K) (V, bool) {
	if m == nil {
		var zero V
		return zero, false
	}
	position, found := m.forward.get(key)
	if !found {
		var zero V
		return zero, false
	}
	return m.entries[position].Value, true
}

func (m *BiMap[K, V]) GetKey(value V) (K, bool) {
	return m.Inverse().GetValue(value)
}

func (m *BiMap[K, V]) ContainsKey(key K) bool {
	_, found := m.GetValue(key)
	return found
}

func (m *BiMap[K, V]) ContainsValue(value V) bool {
	_, found := m.GetKey(value)
	return found
}

func (m *BiMap[K, V]) Entries() iter.Seq[bimap.Association[K, V]] {
	if m == nil {
		return sequence[bimap.Association[K, V]](nil)
	}
	return sequence(m.entries)
}

func (m *BiMap[K, V]) EntriesSlice() []bimap.Association[K, V] {
	if m == nil {
		return nil
	}
	return append([]bimap.Association[K, V]{}, m.entries...)
}

func (m *BiMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

func (m *BiMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

func (m *BiMap[K, V]) Equal(other *BiMap[K, V]) bool {
	if m.Len() != other.Len() {
		return false
	}
	for entry := range m.Entries() {
		value, found := other.GetValue(entry.Key)
		if !found || any(value) != any(entry.Value) {
			return false
		}
	}
	return true
}

// BiMapBuilder incrementally constructs independent BiMap snapshots.
type BiMapBuilder[K any, V any] struct {
	entries    []bimap.Association[K, V]
	keyEqual   func(K, K) bool
	keyHash    func(K) uint64
	valueEqual func(V, V) bool
	valueHash  func(V) uint64
}

func NewBiMapBuilder[K comparable, V comparable]() *BiMapBuilder[K, V] {
	keyEqual, keyHash := naturalFunctions[K]()
	valueEqual, valueHash := naturalFunctions[V]()
	return NewBiMapBuilderFunc(keyEqual, keyHash, valueEqual, valueHash)
}

func NewBiMapBuilderFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool, valueHash func(V) uint64) *BiMapBuilder[K, V] {
	if keyEqual == nil || keyHash == nil || valueEqual == nil || valueHash == nil {
		panic("immutable: NewBiMapBuilderFunc requires non-nil functions")
	}
	return &BiMapBuilder[K, V]{keyEqual: keyEqual, keyHash: keyHash, valueEqual: valueEqual, valueHash: valueHash}
}

func (b *BiMapBuilder[K, V]) Put(key K, value V) {
	b.entries = append(b.entries, bimap.Association[K, V]{Key: key, Value: value})
}

func (b *BiMapBuilder[K, V]) PutAll(entries []bimap.Association[K, V]) {
	b.entries = append(b.entries, entries...)
}

func (b *BiMapBuilder[K, V]) Build() (*BiMap[K, V], error) {
	if b == nil {
		return nil, nil
	}
	return NewBiMapFunc(b.keyEqual, b.keyHash, b.valueEqual, b.valueHash, b.entries)
}
