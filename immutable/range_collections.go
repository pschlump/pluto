/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"iter"

	"github.com/pschlump/pluto/range_map"
	"github.com/pschlump/pluto/range_set"
	"github.com/pschlump/pluto/ranges"
)

// RangeSet is an immutable ordered disjoint-range snapshot.
type RangeSet[T any] struct {
	values    []ranges.Range[T]
	compare   func(T, T) int
	domain    ranges.DiscreteDomain[T]
	hasDomain bool
}

func FreezeRangeSet[T any](source *range_set.RangeSet[T]) *RangeSet[T] {
	if source == nil {
		return nil
	}
	domain, hasDomain := source.Domain()
	return &RangeSet[T]{values: source.Slice(), compare: source.Compare(), domain: domain, hasDomain: hasDomain}
}
func CopyOfRangeSet[T any](value *RangeSet[T]) *RangeSet[T] { return value }
func (s *RangeSet[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}
func (s *RangeSet[T]) IsEmpty() bool { return s.Len() == 0 }
func (s *RangeSet[T]) Slice() []ranges.Range[T] {
	if s == nil {
		return nil
	}
	return append([]ranges.Range[T]{}, s.values...)
}
func (s *RangeSet[T]) All() iter.Seq[ranges.Range[T]] {
	if s == nil {
		return sequence[ranges.Range[T]](nil)
	}
	return sequence(s.values)
}
func (s *RangeSet[T]) Contains(value T) bool { _, found := s.RangeContaining(value); return found }
func (s *RangeSet[T]) RangeContaining(value T) (ranges.Range[T], bool) {
	if s == nil {
		return ranges.Range[T]{}, false
	}
	for _, candidate := range s.values {
		if candidate.Contains(value) {
			return candidate, true
		}
	}
	return ranges.Range[T]{}, false
}
func (s *RangeSet[T]) Encloses(value ranges.Range[T]) bool {
	if value.IsEmpty() {
		return true
	}
	for _, candidate := range s.values {
		if candidate.Encloses(value) {
			return true
		}
	}
	return false
}
func (s *RangeSet[T]) Span() (ranges.Range[T], bool) {
	if s == nil || len(s.values) == 0 {
		return ranges.Range[T]{}, false
	}
	return s.values[0].Span(s.values[len(s.values)-1]), true
}
func (s *RangeSet[T]) mutable() *range_set.RangeSet[T] {
	result := range_set.NewRangeSetFunc(s.compare)
	if s.hasDomain {
		result = range_set.NewDiscreteRangeSetFunc(s.compare, s.domain)
	}
	for _, value := range s.values {
		result.Add(value)
	}
	return result
}
func (s *RangeSet[T]) Complement() *RangeSet[T] {
	if s == nil {
		return nil
	}
	return FreezeRangeSet(s.mutable().Complement())
}
func (s *RangeSet[T]) SubRangeSet(value ranges.Range[T]) *RangeSet[T] {
	if s == nil {
		return nil
	}
	return FreezeRangeSet(s.mutable().SubRangeSet(value))
}
func (s *RangeSet[T]) Equal(other *RangeSet[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for index, value := range s.values {
		if !value.Equal(other.values[index]) {
			return false
		}
	}
	return true
}

// RangeMap is an immutable ordered range/value snapshot.
type RangeMap[T any, V any] struct {
	entries   []range_map.Entry[T, V]
	compare   func(T, T) int
	domain    ranges.DiscreteDomain[T]
	hasDomain bool
}

func FreezeRangeMap[T any, V any](source *range_map.RangeMap[T, V]) *RangeMap[T, V] {
	if source == nil {
		return nil
	}
	domain, hasDomain := source.Domain()
	return &RangeMap[T, V]{entries: source.EntriesSlice(), compare: source.Compare(), domain: domain, hasDomain: hasDomain}
}
func CopyOfRangeMap[T any, V any](value *RangeMap[T, V]) *RangeMap[T, V] { return value }
func (m *RangeMap[T, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}
func (m *RangeMap[T, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *RangeMap[T, V]) EntriesSlice() []range_map.Entry[T, V] {
	if m == nil {
		return nil
	}
	return append([]range_map.Entry[T, V]{}, m.entries...)
}
func (m *RangeMap[T, V]) Entries() iter.Seq[range_map.Entry[T, V]] {
	if m == nil {
		return sequence[range_map.Entry[T, V]](nil)
	}
	return sequence(m.entries)
}
func (m *RangeMap[T, V]) Get(point T) (V, bool) {
	_, value, found := m.RangeContaining(point)
	return value, found
}
func (m *RangeMap[T, V]) RangeContaining(point T) (ranges.Range[T], V, bool) {
	if m == nil {
		return ranges.Range[T]{}, *new(V), false
	}
	for _, entry := range m.entries {
		if entry.Range.Contains(point) {
			return entry.Range, entry.Value, true
		}
	}
	return ranges.Range[T]{}, *new(V), false
}
func (m *RangeMap[T, V]) SubRangeMap(value ranges.Range[T]) *RangeMap[T, V] {
	if m == nil {
		return nil
	}
	mutable := range_map.NewRangeMapFunc[T, V](m.compare)
	if m.hasDomain {
		mutable = range_map.NewDiscreteRangeMapFunc[T, V](m.compare, m.domain)
	}
	for _, entry := range m.entries {
		mutable.Put(entry.Range, entry.Value)
	}
	return FreezeRangeMap(mutable.SubRangeMap(value))
}
func (m *RangeMap[T, V]) Equal(other *RangeMap[T, V], equal func(V, V) bool) bool {
	if m.Len() != other.Len() {
		return false
	}
	if equal == nil {
		panic("immutable: nil RangeMap value equality")
	}
	for index, entry := range m.entries {
		if !entry.Range.Equal(other.entries[index].Range) || !equal(entry.Value, other.entries[index].Value) {
			return false
		}
	}
	return true
}
