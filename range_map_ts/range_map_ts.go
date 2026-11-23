/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package range_map_ts is the mutex-guarded twin of range_map. Iterators and
// derived maps are snapshots and opposite-way equality never holds two locks.
package range_map_ts

import (
	"cmp"
	"iter"
	"slices"
	"sync"

	"github.com/pschlump/pluto/range_map"
	"github.com/pschlump/pluto/ranges"
)

type RangeMap[T any, V any] struct {
	lock  sync.RWMutex
	plain *range_map.RangeMap[T, V]
}

func NewRangeMap[T cmp.Ordered, V any]() *RangeMap[T, V] {
	return &RangeMap[T, V]{plain: range_map.NewRangeMap[T, V]()}
}
func NewRangeMapFunc[T any, V any](compare func(T, T) int) *RangeMap[T, V] {
	return &RangeMap[T, V]{plain: range_map.NewRangeMapFunc[T, V](compare)}
}
func NewDiscreteRangeMap[T cmp.Ordered, V any](domain ranges.DiscreteDomain[T]) *RangeMap[T, V] {
	return &RangeMap[T, V]{plain: range_map.NewDiscreteRangeMap[T, V](domain)}
}
func NewDiscreteRangeMapFunc[T any, V any](compare func(T, T) int, domain ranges.DiscreteDomain[T]) *RangeMap[T, V] {
	return &RangeMap[T, V]{plain: range_map.NewDiscreteRangeMapFunc[T, V](compare, domain)}
}
func wrap[T any, V any](plain *range_map.RangeMap[T, V]) *RangeMap[T, V] {
	if plain == nil {
		return nil
	}
	return &RangeMap[T, V]{plain: plain}
}
func (m *RangeMap[T, V]) Put(value ranges.Range[T], mapped V) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.plain.Put(value, mapped)
}
func (m *RangeMap[T, V]) Remove(value ranges.Range[T]) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.plain.Remove(value)
}
func (m *RangeMap[T, V]) Len() int {
	if m == nil || m.plain == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.plain.Len()
}
func (m *RangeMap[T, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *RangeMap[T, V]) Clear() {
	if m == nil || m.plain == nil {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.plain.Clear()
}
func (m *RangeMap[T, V]) Compare() func(T, T) int {
	if m == nil || m.plain == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.plain.Compare()
}
func (m *RangeMap[T, V]) EntriesSlice() []range_map.Entry[T, V] {
	if m == nil || m.plain == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.plain.EntriesSlice()
}
func (m *RangeMap[T, V]) Entries() iter.Seq[range_map.Entry[T, V]] {
	return slices.Values(m.EntriesSlice())
}
func (m *RangeMap[T, V]) Get(point T) (V, bool) {
	if m == nil || m.plain == nil {
		return *new(V), false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.plain.Get(point)
}
func (m *RangeMap[T, V]) RangeContaining(point T) (ranges.Range[T], V, bool) {
	if m == nil || m.plain == nil {
		return ranges.Range[T]{}, *new(V), false
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.plain.RangeContaining(point)
}
func (m *RangeMap[T, V]) SubRangeMap(value ranges.Range[T]) *RangeMap[T, V] {
	if m == nil || m.plain == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return wrap(m.plain.SubRangeMap(value))
}
func (m *RangeMap[T, V]) Clone() *RangeMap[T, V] {
	if m == nil || m.plain == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return wrap(m.plain.Clone())
}
func (m *RangeMap[T, V]) Equal(other *RangeMap[T, V], equal func(V, V) bool) bool {
	left, right := m.EntriesSlice(), other.EntriesSlice()
	if len(left) != len(right) {
		return false
	}
	if equal == nil {
		panic("range_map_ts: nil value equality")
	}
	for index := range left {
		if !left[index].Range.Equal(right[index].Range) || !equal(left[index].Value, right[index].Value) {
			return false
		}
	}
	return true
}
