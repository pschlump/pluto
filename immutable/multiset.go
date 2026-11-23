/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"iter"

	"github.com/pschlump/pluto/multiset"
)

const maxInt = int(^uint(0) >> 1)

// Multiset is an immutable count-aware collection with first-distinct order for
// hash sources and comparator order for tree sources.
type Multiset[T any] struct {
	entries []multiset.Entry[T]
	total   int
	index   *hashIndex[T]
	compare func(T, T) int
	equal   func(T, T) bool
	hash    func(T) uint64
}

func NewMultiset[T comparable](values []T) *Multiset[T] {
	equal, hash := naturalFunctions[T]()
	return NewMultisetFunc(equal, hash, values)
}

func NewMultisetFunc[T any](equal func(T, T) bool, hash func(T) uint64, values []T) *Multiset[T] {
	result := &Multiset[T]{index: newHashIndex(equal, hash), equal: equal, hash: hash}
	for _, value := range values {
		position, found := result.index.get(value)
		if !found {
			position = len(result.entries)
			result.index.add(value, position)
			result.entries = append(result.entries, multiset.Entry[T]{Value: value})
		}
		if result.entries[position].Count == maxInt || result.total == maxInt {
			panic("immutable: multiset count or size overflow")
		}
		result.entries[position].Count++
		result.total++
	}
	return result
}

func CopyOfMultiset[T any](value *Multiset[T]) *Multiset[T] { return value }

// FreezeHashMultiset takes an independent snapshot retaining custom functions.
func FreezeHashMultiset[T any](source *multiset.HashMultiset[T]) *Multiset[T] {
	if source == nil {
		return nil
	}
	if source.Eq() == nil {
		return &Multiset[T]{}
	}
	entries := source.EntriesSlice()
	result := &Multiset[T]{index: newHashIndex(source.Eq(), source.Hash()), equal: source.Eq(), hash: source.Hash(), entries: entries}
	for position, entry := range entries {
		if entry.Count <= 0 || entry.Count > maxInt-result.total {
			panic("immutable: invalid or overflowing multiset counts")
		}
		result.total += entry.Count
		result.index.add(entry.Value, position)
	}
	return result
}

// FreezeTreeMultiset takes an independent sorted snapshot.
func FreezeTreeMultiset[T any](source *multiset.TreeMultiset[T]) *Multiset[T] {
	if source == nil {
		return nil
	}
	result := &Multiset[T]{entries: source.EntriesSlice(), compare: source.Compare()}
	for _, entry := range result.entries {
		if entry.Count <= 0 || entry.Count > maxInt-result.total {
			panic("immutable: invalid or overflowing multiset counts")
		}
		result.total += entry.Count
	}
	return result
}

func (m *Multiset[T]) Len() int {
	if m == nil {
		return 0
	}
	return m.total
}

func (m *Multiset[T]) DistinctLen() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

func (m *Multiset[T]) IsEmpty() bool { return m.Len() == 0 }

func (m *Multiset[T]) Count(value T) int {
	if m == nil {
		return 0
	}
	if m.index != nil {
		position, found := m.index.get(value)
		if !found {
			return 0
		}
		return m.entries[position].Count
	}
	if m.compare == nil {
		return 0
	}
	position := lowerBound(m.entries, value, func(entry multiset.Entry[T], probe T) int {
		return m.compare(entry.Value, probe)
	})
	if position < len(m.entries) && m.compare(m.entries[position].Value, value) == 0 {
		return m.entries[position].Count
	}
	return 0
}

func (m *Multiset[T]) Contains(value T) bool { return m.Count(value) > 0 }

func (m *Multiset[T]) Entries() iter.Seq[multiset.Entry[T]] {
	if m == nil {
		return sequence[multiset.Entry[T]](nil)
	}
	return sequence(m.entries)
}

func (m *Multiset[T]) EntriesSlice() []multiset.Entry[T] {
	if m == nil {
		return nil
	}
	return append([]multiset.Entry[T]{}, m.entries...)
}

func (m *Multiset[T]) Elements() iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range m.Entries() {
			if !yield(entry.Value) {
				return
			}
		}
	}
}

func (m *Multiset[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for entry := range m.Entries() {
			for range entry.Count {
				if !yield(entry.Value) {
					return
				}
			}
		}
	}
}

func (m *Multiset[T]) Slice() []T {
	if m == nil {
		return nil
	}
	values := make([]T, 0, m.total)
	for value := range m.All() {
		values = append(values, value)
	}
	return values
}

func (m *Multiset[T]) Equal(other *Multiset[T]) bool {
	if m.Len() != other.Len() || m.DistinctLen() != other.DistinctLen() {
		return false
	}
	for entry := range m.Entries() {
		if other.Count(entry.Value) != entry.Count {
			return false
		}
	}
	return true
}

// MultisetBuilder incrementally constructs independent hash Multiset snapshots.
type MultisetBuilder[T any] struct {
	values []T
	equal  func(T, T) bool
	hash   func(T) uint64
}

func NewMultisetBuilder[T comparable]() *MultisetBuilder[T] {
	equal, hash := naturalFunctions[T]()
	return NewMultisetBuilderFunc(equal, hash)
}

func NewMultisetBuilderFunc[T any](equal func(T, T) bool, hash func(T) uint64) *MultisetBuilder[T] {
	if equal == nil || hash == nil {
		panic("immutable: NewMultisetBuilderFunc requires non-nil functions")
	}
	return &MultisetBuilder[T]{equal: equal, hash: hash}
}

func (b *MultisetBuilder[T]) Add(value T)       { b.values = append(b.values, value) }
func (b *MultisetBuilder[T]) AddAll(values []T) { b.values = append(b.values, values...) }

func (b *MultisetBuilder[T]) Build() *Multiset[T] {
	if b == nil {
		return nil
	}
	return NewMultisetFunc(b.equal, b.hash, b.values)
}
