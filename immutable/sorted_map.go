/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
)

// SortedMap is an immutable comparator-ordered map with random access.
type SortedMap[K any, V any] struct {
	entries []Entry[K, V]
	compare func(K, K) int
}

func NewSortedMap[K cmp.Ordered, V any](entries []Entry[K, V]) (*SortedMap[K, V], error) {
	return NewSortedMapFunc(cmp.Compare[K], entries)
}

func NewSortedMapFunc[K any, V any](compare func(K, K) int, entries []Entry[K, V]) (*SortedMap[K, V], error) {
	if compare == nil {
		panic("immutable: NewSortedMapFunc called with nil comparison")
	}
	ordered := append([]Entry[K, V](nil), entries...)
	slices.SortStableFunc(ordered, func(a, b Entry[K, V]) int { return compare(a.Key, b.Key) })
	for index := 1; index < len(ordered); index++ {
		if compare(ordered[index-1].Key, ordered[index].Key) == 0 {
			return nil, fmt.Errorf("%w at sorted entry %d", ErrDuplicateKey, index)
		}
	}
	return &SortedMap[K, V]{entries: slices.Clip(ordered), compare: compare}, nil
}

func CopyOfSortedMap[K any, V any](value *SortedMap[K, V]) *SortedMap[K, V] { return value }

func (m *SortedMap[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

func (m *SortedMap[K, V]) IsEmpty() bool { return m.Len() == 0 }

func (m *SortedMap[K, V]) rank(key K) int {
	if m == nil || m.compare == nil {
		return 0
	}
	low, high := 0, len(m.entries)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if m.compare(m.entries[middle].Key, key) < 0 {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}

// Rank returns the number of keys strictly less than key.
func (m *SortedMap[K, V]) Rank(key K) int { return m.rank(key) }

func (m *SortedMap[K, V]) At(index int) (Entry[K, V], bool) {
	if m == nil || index < 0 || index >= len(m.entries) {
		return Entry[K, V]{}, false
	}
	return m.entries[index], true
}

func (m *SortedMap[K, V]) Get(key K) (V, bool) {
	if m == nil || m.compare == nil {
		var zero V
		return zero, false
	}
	position := m.rank(key)
	if position >= len(m.entries) || m.compare(m.entries[position].Key, key) != 0 {
		var zero V
		return zero, false
	}
	return m.entries[position].Value, true
}

func (m *SortedMap[K, V]) ContainsKey(key K) bool {
	_, found := m.Get(key)
	return found
}

func (m *SortedMap[K, V]) navigation(key K, lower, allowEqual bool) (Entry[K, V], bool) {
	if m == nil || m.compare == nil {
		return Entry[K, V]{}, false
	}
	position := m.rank(key)
	if lower {
		if position < len(m.entries) && allowEqual && m.compare(m.entries[position].Key, key) == 0 {
			// Floor uses the equal position.
		} else {
			position--
		}
	} else if !allowEqual && position < len(m.entries) && m.compare(m.entries[position].Key, key) == 0 {
		position++
	}
	return m.At(position)
}

func (m *SortedMap[K, V]) Lower(key K) (Entry[K, V], bool)  { return m.navigation(key, true, false) }
func (m *SortedMap[K, V]) Floor(key K) (Entry[K, V], bool)  { return m.navigation(key, true, true) }
func (m *SortedMap[K, V]) Ceil(key K) (Entry[K, V], bool)   { return m.navigation(key, false, true) }
func (m *SortedMap[K, V]) Higher(key K) (Entry[K, V], bool) { return m.navigation(key, false, false) }

func (m *SortedMap[K, V]) SubRange(from K, fromInclusive bool, to K, toInclusive bool) *SortedMap[K, V] {
	if m == nil || m.compare == nil || m.compare(from, to) > 0 {
		panic("immutable: invalid SortedMap subrange")
	}
	start := m.rank(from)
	if !fromInclusive && start < len(m.entries) && m.compare(m.entries[start].Key, from) == 0 {
		start++
	}
	end := m.rank(to)
	if toInclusive && end < len(m.entries) && m.compare(m.entries[end].Key, to) == 0 {
		end++
	}
	if start > end {
		start = end
	}
	return &SortedMap[K, V]{entries: copySubslice(m.entries[start:end]), compare: m.compare}
}

func (m *SortedMap[K, V]) Head(to K, inclusive bool) *SortedMap[K, V] {
	if m == nil {
		return &SortedMap[K, V]{}
	}
	end := m.rank(to)
	if inclusive && end < len(m.entries) && m.compare(m.entries[end].Key, to) == 0 {
		end++
	}
	return &SortedMap[K, V]{entries: copySubslice(m.entries[:end]), compare: m.compare}
}

func (m *SortedMap[K, V]) Tail(from K, inclusive bool) *SortedMap[K, V] {
	if m == nil {
		return &SortedMap[K, V]{}
	}
	start := m.rank(from)
	if !inclusive && start < len(m.entries) && m.compare(m.entries[start].Key, from) == 0 {
		start++
	}
	return &SortedMap[K, V]{entries: copySubslice(m.entries[start:]), compare: m.compare}
}

func (m *SortedMap[K, V]) Entries() iter.Seq[Entry[K, V]] {
	if m == nil {
		return sequence[Entry[K, V]](nil)
	}
	return sequence(m.entries)
}

func (m *SortedMap[K, V]) EntriesSlice() []Entry[K, V] {
	if m == nil {
		return nil
	}
	return append([]Entry[K, V]{}, m.entries...)
}

func (m *SortedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Key) {
				return
			}
		}
	}
}

func (m *SortedMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

func (m *SortedMap[K, V]) Reverse() iter.Seq[Entry[K, V]] {
	if m == nil {
		return sequence[Entry[K, V]](nil)
	}
	return reverseSequence(m.entries)
}

func (m *SortedMap[K, V]) Equal(other *SortedMap[K, V]) bool {
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

// SortedMapBuilder incrementally constructs independent SortedMap snapshots.
type SortedMapBuilder[K any, V any] struct {
	entries []Entry[K, V]
	compare func(K, K) int
}

func NewSortedMapBuilder[K cmp.Ordered, V any]() *SortedMapBuilder[K, V] {
	return NewSortedMapBuilderFunc[K, V](cmp.Compare[K])
}

func NewSortedMapBuilderFunc[K any, V any](compare func(K, K) int) *SortedMapBuilder[K, V] {
	if compare == nil {
		panic("immutable: NewSortedMapBuilderFunc called with nil comparison")
	}
	return &SortedMapBuilder[K, V]{compare: compare}
}

func (b *SortedMapBuilder[K, V]) Put(key K, value V) {
	b.entries = append(b.entries, Entry[K, V]{Key: key, Value: value})
}

func (b *SortedMapBuilder[K, V]) PutAll(entries []Entry[K, V]) {
	b.entries = append(b.entries, entries...)
}

func (b *SortedMapBuilder[K, V]) Build() (*SortedMap[K, V], error) {
	if b == nil {
		return nil, nil
	}
	return NewSortedMapFunc(b.compare, b.entries)
}
