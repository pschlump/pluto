/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"iter"

	"github.com/pschlump/pluto/multimap"
)

// Multimap is an immutable list multimap. Keys retain first occurrence order and
// each key's values retain input order, including duplicates.
type Multimap[K any, V any] struct {
	keys       []K
	values     [][]V
	total      int
	index      *hashIndex[K]
	valueEqual func(V, V) bool
	keyEqual   func(K, K) bool
	keyHash    func(K) uint64
}

func NewMultimap[K comparable, V comparable](entries []multimap.Association[K, V]) *Multimap[K, V] {
	keyEqual, keyHash := naturalFunctions[K]()
	return NewMultimapFunc(keyEqual, keyHash, func(a, b V) bool { return a == b }, entries)
}

func NewMultimapFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool, entries []multimap.Association[K, V]) *Multimap[K, V] {
	if valueEqual == nil {
		panic("immutable: NewMultimapFunc requires non-nil value equality")
	}
	result := &Multimap[K, V]{index: newHashIndex(keyEqual, keyHash), valueEqual: valueEqual, keyEqual: keyEqual, keyHash: keyHash}
	for _, entry := range entries {
		if result.total == maxInt {
			panic("immutable: multimap association count overflow")
		}
		position, found := result.index.get(entry.Key)
		if !found {
			position = len(result.keys)
			result.index.add(entry.Key, position)
			result.keys = append(result.keys, entry.Key)
			result.values = append(result.values, nil)
		}
		result.values[position] = append(result.values[position], entry.Value)
		result.total++
	}
	return result
}

func CopyOfMultimap[K any, V any](value *Multimap[K, V]) *Multimap[K, V] { return value }

func FreezeListMultimap[K any, V any](source *multimap.ListMultimap[K, V]) *Multimap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		return &Multimap[K, V]{valueEqual: source.ValueEq()}
	}
	return NewMultimapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.EntriesSlice())
}

func FreezeSetMultimap[K any, V any](source *multimap.SetMultimap[K, V]) *Multimap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		return &Multimap[K, V]{valueEqual: source.ValueEq()}
	}
	return NewMultimapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.EntriesSlice())
}

func FreezeSortedSetMultimap[K any, V any](source *multimap.SortedSetMultimap[K, V]) *Multimap[K, V] {
	if source == nil {
		return nil
	}
	if source.KeyEq() == nil {
		return &Multimap[K, V]{valueEqual: source.ValueEq()}
	}
	return NewMultimapFunc(source.KeyEq(), source.KeyHash(), source.ValueEq(), source.EntriesSlice())
}

func (m *Multimap[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return m.total
}

func (m *Multimap[K, V]) KeyLen() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

func (m *Multimap[K, V]) IsEmpty() bool { return m.Len() == 0 }

func (m *Multimap[K, V]) Get(key K) ([]V, bool) {
	if m == nil {
		return nil, false
	}
	position, found := m.index.get(key)
	if !found {
		return nil, false
	}
	return append([]V{}, m.values[position]...), true
}

func (m *Multimap[K, V]) ContainsKey(key K) bool {
	_, found := m.Get(key)
	return found
}

func (m *Multimap[K, V]) ContainsEntry(key K, value V) bool {
	values, found := m.Get(key)
	if !found {
		return false
	}
	for _, candidate := range values {
		if m.valueEqual(candidate, value) {
			return true
		}
	}
	return false
}

func (m *Multimap[K, V]) ContainsValue(value V) bool {
	for candidate := range m.Values() {
		if m.valueEqual(candidate, value) {
			return true
		}
	}
	return false
}

func (m *Multimap[K, V]) Entries() iter.Seq[multimap.Association[K, V]] {
	return func(yield func(multimap.Association[K, V]) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			for _, value := range m.values[position] {
				if !yield(multimap.Association[K, V]{Key: key, Value: value}) {
					return
				}
			}
		}
	}
}

func (m *Multimap[K, V]) EntriesSlice() []multimap.Association[K, V] {
	entries := []multimap.Association[K, V]{}
	for entry := range m.Entries() {
		entries = append(entries, entry)
	}
	return entries
}

func (m *Multimap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			for range m.values[position] {
				if !yield(key) {
					return
				}
			}
		}
	}
}

func (m *Multimap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		if m == nil {
			return
		}
		for _, values := range m.values {
			for _, value := range values {
				if !yield(value) {
					return
				}
			}
		}
	}
}

func (m *Multimap[K, V]) KeySet() iter.Seq[K] {
	if m == nil {
		return sequence[K](nil)
	}
	return sequence(m.keys)
}

// AsMap yields each key and a fresh value slice snapshot.
func (m *Multimap[K, V]) AsMap() iter.Seq2[K, []V] {
	return func(yield func(K, []V) bool) {
		if m == nil {
			return
		}
		for position, key := range m.keys {
			if !yield(key, append([]V{}, m.values[position]...)) {
				return
			}
		}
	}
}

func (m *Multimap[K, V]) Equal(other *Multimap[K, V]) bool {
	if m.Len() != other.Len() || m.KeyLen() != other.KeyLen() {
		return false
	}
	for position, key := range m.keys {
		values, found := other.Get(key)
		if !found || len(values) != len(m.values[position]) {
			return false
		}
		for index, value := range m.values[position] {
			if !m.valueEqual(value, values[index]) {
				return false
			}
		}
	}
	return true
}

// MultimapBuilder incrementally constructs independent Multimap snapshots.
type MultimapBuilder[K any, V any] struct {
	entries    []multimap.Association[K, V]
	keyEqual   func(K, K) bool
	keyHash    func(K) uint64
	valueEqual func(V, V) bool
}

func NewMultimapBuilder[K comparable, V comparable]() *MultimapBuilder[K, V] {
	keyEqual, keyHash := naturalFunctions[K]()
	return NewMultimapBuilderFunc(keyEqual, keyHash, func(a, b V) bool { return a == b })
}

func NewMultimapBuilderFunc[K any, V any](keyEqual func(K, K) bool, keyHash func(K) uint64, valueEqual func(V, V) bool) *MultimapBuilder[K, V] {
	if keyEqual == nil || keyHash == nil || valueEqual == nil {
		panic("immutable: NewMultimapBuilderFunc requires non-nil functions")
	}
	return &MultimapBuilder[K, V]{keyEqual: keyEqual, keyHash: keyHash, valueEqual: valueEqual}
}

func (b *MultimapBuilder[K, V]) Put(key K, value V) {
	b.entries = append(b.entries, multimap.Association[K, V]{Key: key, Value: value})
}

func (b *MultimapBuilder[K, V]) PutAll(entries []multimap.Association[K, V]) {
	b.entries = append(b.entries, entries...)
}

func (b *MultimapBuilder[K, V]) Build() *Multimap[K, V] {
	if b == nil {
		return nil
	}
	return NewMultimapFunc(b.keyEqual, b.keyHash, b.valueEqual, b.entries)
}
