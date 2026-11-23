/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package range_map implements ordered mappings from disjoint ranges to values.
// Adjacent mappings are not coalesced, even when their values compare equal.
package range_map

import (
	"cmp"
	"iter"
	"sort"

	"github.com/pschlump/pluto/ranges"
)

// Entry associates a range with a value.
type Entry[T any, V any] struct {
	Range ranges.Range[T]
	Value V
}

// RangeMap stores sorted, non-overlapping range/value associations.
type RangeMap[T any, V any] struct {
	entries   []Entry[T, V]
	compare   func(T, T) int
	domain    ranges.DiscreteDomain[T]
	hasDomain bool
}

func NewRangeMap[T cmp.Ordered, V any]() *RangeMap[T, V] {
	return NewRangeMapFunc[T, V](cmp.Compare[T])
}
func NewRangeMapFunc[T any, V any](compare func(T, T) int) *RangeMap[T, V] {
	if compare == nil {
		panic("range_map: nil comparison function")
	}
	return &RangeMap[T, V]{compare: compare}
}
func NewDiscreteRangeMap[T cmp.Ordered, V any](domain ranges.DiscreteDomain[T]) *RangeMap[T, V] {
	return NewDiscreteRangeMapFunc[T, V](cmp.Compare[T], domain)
}
func NewDiscreteRangeMapFunc[T any, V any](compare func(T, T) int, domain ranges.DiscreteDomain[T]) *RangeMap[T, V] {
	result := NewRangeMapFunc[T, V](compare)
	result.domain, result.hasDomain = domain, true
	return result
}

func (m *RangeMap[T, V]) Compare() func(T, T) int {
	if m == nil {
		return nil
	}
	return m.compare
}
func (m *RangeMap[T, V]) Domain() (ranges.DiscreteDomain[T], bool) {
	if m == nil || !m.hasDomain {
		return ranges.DiscreteDomain[T]{}, false
	}
	return m.domain, true
}
func (m *RangeMap[T, V]) canonical(value ranges.Range[T]) ranges.Range[T] {
	if m.hasDomain {
		return value.Canonical(m.domain)
	}
	return value
}
func (m *RangeMap[T, V]) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}
func (m *RangeMap[T, V]) IsEmpty() bool { return m.Len() == 0 }
func (m *RangeMap[T, V]) Clear() {
	if m != nil {
		m.entries = nil
	}
}
func (m *RangeMap[T, V]) EntriesSlice() []Entry[T, V] {
	if m == nil {
		return nil
	}
	return append([]Entry[T, V]{}, m.entries...)
}
func (m *RangeMap[T, V]) Entries() iter.Seq[Entry[T, V]] {
	entries := m.EntriesSlice()
	return func(yield func(Entry[T, V]) bool) {
		for _, entry := range entries {
			if !yield(entry) {
				return
			}
		}
	}
}

func lowerCompare[T any, V any](a Entry[T, V], b ranges.Range[T]) int {
	aLower, aBound, aHas := a.Range.LowerBound()
	bLower, bBound, bHas := b.LowerBound()
	if !aHas {
		if bHas {
			return -1
		}
		return 0
	}
	if !bHas {
		return 1
	}
	order := a.Range.Compare()(aLower, bLower)
	if order == 0 && aBound != bBound {
		if aBound == ranges.OpenBound {
			return 1
		}
		return -1
	}
	return order
}

func makeBounds[T any](compare func(T, T) int, lower T, lowerOpen, hasLower bool, upper T, upperOpen, hasUpper bool) ranges.Range[T] {
	switch {
	case hasLower && hasUpper:
		switch {
		case lowerOpen && upperOpen:
			return ranges.OpenFunc(compare, lower, upper)
		case lowerOpen:
			return ranges.OpenClosedFunc(compare, lower, upper)
		case upperOpen:
			return ranges.ClosedOpenFunc(compare, lower, upper)
		default:
			return ranges.ClosedFunc(compare, lower, upper)
		}
	case hasLower:
		if lowerOpen {
			return ranges.GreaterThanFunc(compare, lower)
		}
		return ranges.AtLeastFunc(compare, lower)
	case hasUpper:
		if upperOpen {
			return ranges.LessThanFunc(compare, upper)
		}
		return ranges.AtMostFunc(compare, upper)
	default:
		return ranges.AllFunc(compare)
	}
}

func (m *RangeMap[T, V]) makeBounds(lower T, lowerOpen, hasLower bool, upper T, upperOpen, hasUpper bool) ranges.Range[T] {
	return m.canonical(makeBounds(m.compare, lower, lowerOpen, hasLower, upper, upperOpen, hasUpper))
}

// Put replaces all mappings overlapping value, preserving non-overlapping left
// and right fragments of partially covered entries.
func (m *RangeMap[T, V]) Put(value ranges.Range[T], mapped V) {
	if m == nil || m.compare == nil {
		panic("range_map: Put on zero-value RangeMap")
	}
	value = m.canonical(value)
	if value.IsEmpty() {
		return
	}
	result := make([]Entry[T, V], 0, len(m.entries)+1)
	for _, existing := range m.entries {
		if !existing.Range.IsConnected(value) || existing.Range.Intersection(value).IsEmpty() {
			result = append(result, existing)
			continue
		}
		oldLower, oldLowerBound, oldHasLower := existing.Range.LowerBound()
		oldUpper, oldUpperBound, oldHasUpper := existing.Range.UpperBound()
		cutLower, cutLowerBound, cutHasLower := value.LowerBound()
		cutUpper, cutUpperBound, cutHasUpper := value.UpperBound()
		if cutHasLower {
			order := m.compare(oldLower, cutLower)
			left := !oldHasLower || order < 0 || order == 0 && oldLowerBound == ranges.ClosedBound && cutLowerBound == ranges.OpenBound
			if left {
				result = append(result, Entry[T, V]{Range: m.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true), Value: existing.Value})
			}
		}
		if cutHasUpper {
			order := m.compare(cutUpper, oldUpper)
			right := !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
			if right {
				result = append(result, Entry[T, V]{Range: m.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper), Value: existing.Value})
			}
		}
	}
	result = append(result, Entry[T, V]{Range: value, Value: mapped})
	sort.SliceStable(result, func(i, j int) bool { return lowerCompare(result[i], result[j].Range) < 0 })
	m.entries = result
}

// Remove deletes value's interval and preserves exact boundary fragments.
func (m *RangeMap[T, V]) Remove(value ranges.Range[T]) {
	if m == nil || m.compare == nil {
		panic("range_map: Remove on zero-value RangeMap")
	}
	value = m.canonical(value)
	if value.IsEmpty() {
		return
	}
	result := make([]Entry[T, V], 0, len(m.entries))
	for _, existing := range m.entries {
		if !existing.Range.IsConnected(value) || existing.Range.Intersection(value).IsEmpty() {
			result = append(result, existing)
			continue
		}
		oldLower, oldLowerBound, oldHasLower := existing.Range.LowerBound()
		oldUpper, oldUpperBound, oldHasUpper := existing.Range.UpperBound()
		cutLower, cutLowerBound, cutHasLower := value.LowerBound()
		cutUpper, cutUpperBound, cutHasUpper := value.UpperBound()
		if cutHasLower {
			order := m.compare(oldLower, cutLower)
			left := !oldHasLower || order < 0 || order == 0 && oldLowerBound == ranges.ClosedBound && cutLowerBound == ranges.OpenBound
			if left {
				result = append(result, Entry[T, V]{Range: m.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true), Value: existing.Value})
			}
		}
		if cutHasUpper {
			order := m.compare(cutUpper, oldUpper)
			right := !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
			if right {
				result = append(result, Entry[T, V]{Range: m.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper), Value: existing.Value})
			}
		}
	}
	m.entries = result
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

// SubRangeMap returns an independent snapshot intersected with value.
func (m *RangeMap[T, V]) SubRangeMap(value ranges.Range[T]) *RangeMap[T, V] {
	result := NewRangeMapFunc[T, V](m.compare)
	result.domain, result.hasDomain = m.domain, m.hasDomain
	for _, entry := range m.entries {
		if entry.Range.IsConnected(value) {
			intersection := entry.Range.Intersection(value)
			if !intersection.IsEmpty() {
				result.entries = append(result.entries, Entry[T, V]{Range: intersection, Value: entry.Value})
			}
		}
	}
	return result
}
func (m *RangeMap[T, V]) Clone() *RangeMap[T, V] {
	if m == nil {
		return nil
	}
	return &RangeMap[T, V]{entries: m.EntriesSlice(), compare: m.compare, domain: m.domain, hasDomain: m.hasDomain}
}
func (m *RangeMap[T, V]) Equal(other *RangeMap[T, V], equal func(V, V) bool) bool {
	if m.Len() != other.Len() {
		return false
	}
	if equal == nil {
		panic("range_map: nil value equality")
	}
	for index, entry := range m.entries {
		if !entry.Range.Equal(other.entries[index].Range) || !equal(entry.Value, other.entries[index].Value) {
			return false
		}
	}
	return true
}
