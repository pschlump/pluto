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
//
// Entries are stored as parallel key and value slices (entry i is
// keys[i]/values[i]) so the inverse shares both slices, swapped, instead of
// holding its own swapped copy of every entry.
type BiMap[K any, V any] struct {
	keys    []K
	values  []V
	forward *hashIndex[K]
	reverse *hashIndex[V]
	inverse *BiMap[V, K]
}

func NewBiMap[K comparable, V comparable](entries []bimap.Association[K, V]) (*BiMap[K, V], error) {
	return newBiMap(newNaturalIndex[K](len(entries)), newNaturalIndex[V](len(entries)), entries)
}

func NewBiMapFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool, valueHash func(V) uint64, entries []bimap.Association[K, V]) (*BiMap[K, V], error) {
	return newBiMap(newHashIndexSized(keyEqual, keyHash, len(entries)), newHashIndexSized(valueEqual, valueHash, len(entries)), entries)
}

func newBiMap[K any, V any](forward *hashIndex[K], reverse *hashIndex[V], entries []bimap.Association[K, V]) (*BiMap[K, V], error) {
	for position, entry := range entries {
		if !forward.addUnique(entry.Key, position) {
			return nil, fmt.Errorf("%w at entry %d", ErrDuplicateKey, position)
		}
		if !reverse.addUnique(entry.Value, position) {
			return nil, fmt.Errorf("%w at entry %d", ErrDuplicateValue, position)
		}
	}
	result := &BiMap[K, V]{forward: forward, reverse: reverse}
	if len(entries) > 0 {
		result.keys = make([]K, len(entries))
		result.values = make([]V, len(entries))
		for position, entry := range entries {
			result.keys[position] = entry.Key
			result.values[position] = entry.Value
		}
	}
	inverse := &BiMap[V, K]{keys: result.values, values: result.keys, forward: reverse, reverse: forward}
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
	return len(m.keys)
}

func (m *BiMap[K, V]) IsEmpty() bool { return m.Len() == 0 }

// GetValue returns key's value. Complexity is expected O(1).
func (m *BiMap[K, V]) GetValue(key K) (value V, found bool) {
	if m == nil {
		return value, false
	}
	position, found := m.forward.get(key)
	if found {
		value = m.values[position]
	}
	return value, found
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
	return func(yield func(bimap.Association[K, V]) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			if !yield(bimap.Association[K, V]{Key: key, Value: m.values[position]}) {
				return
			}
		}
	}
}

func (m *BiMap[K, V]) EntriesSlice() []bimap.Association[K, V] {
	if m == nil {
		return nil
	}
	entries := make([]bimap.Association[K, V], len(m.keys))
	for position, key := range m.keys {
		entries[position] = bimap.Association[K, V]{Key: key, Value: m.values[position]}
	}
	return entries
}

func (m *BiMap[K, V]) Keys() iter.Seq[K] {
	var values []K
	if m != nil {
		values = m.keys
	}
	return sequence(values)
}

func (m *BiMap[K, V]) Values() iter.Seq[V] {
	var values []V
	if m != nil {
		values = m.values
	}
	return sequence(values)
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
	entries      []bimap.Association[K, V]
	keyEqual     func(K, K) bool
	keyHash      func(K) uint64
	valueEqual   func(V, V) bool
	valueHash    func(V) uint64
	naturalKey   func(hint int) *hashIndex[K] // set by NewBiMapBuilder only
	naturalValue func(hint int) *hashIndex[V] // set by NewBiMapBuilder only
}

func NewBiMapBuilder[K comparable, V comparable]() *BiMapBuilder[K, V] {
	keyEqual, keyHash := naturalFunctions[K]()
	valueEqual, valueHash := naturalFunctions[V]()
	builder := NewBiMapBuilderFunc(keyEqual, keyHash, valueEqual, valueHash)
	builder.naturalKey, builder.naturalValue = newNaturalIndex[K], newNaturalIndex[V]
	return builder
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
	if b.naturalKey != nil {
		return newBiMap(b.naturalKey(len(b.entries)), b.naturalValue(len(b.entries)), b.entries)
	}
	return NewBiMapFunc(b.keyEqual, b.keyHash, b.valueEqual, b.valueHash, b.entries)
}
