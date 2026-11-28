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
	"slices"

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

// lowerAfterUpper orders left's lower bound against right's upper bound under
// compare: positive when left's lower bound lies strictly after right's upper
// bound, otherwise zero or negative. A missing bound is infinite, and touching
// open endpoints count as strictly after — exactly one half of
// ranges.Range.IsConnected.
func lowerAfterUpper[T any](compare func(T, T) int, left, right ranges.Range[T]) int {
	lower, lowerBound, hasLower := left.LowerBound()
	upper, upperBound, hasUpper := right.UpperBound()
	if !hasLower || !hasUpper {
		return -1
	}
	order := compare(lower, upper)
	if order == 0 && lowerBound == ranges.OpenBound && upperBound == ranges.OpenBound {
		return 1
	}
	return order
}

// connectedWindow returns the half-open index window [lo, hi) of the entries
// whose ranges are connected to value. Entry ranges are sorted and disjoint
// (though possibly touching), so their upper bounds ascend with their lower
// bounds and both halves of IsConnected are monotone over the slice: the
// connected entries are one contiguous run, found by two binary searches.
// Complexity is O(log r).
func (m *RangeMap[T, V]) connectedWindow(value ranges.Range[T]) (int, int) {
	entries := m.entries
	// lo: the first entry whose upper bound reaches value's lower bound.
	lo, end := 0, len(entries)
	for lo < end {
		middle := int(uint(lo+end) >> 1)
		if lowerAfterUpper(m.compare, value, entries[middle].Range) > 0 {
			lo = middle + 1
		} else {
			end = middle
		}
	}
	// hi: the first entry at or after lo whose lower bound lies strictly after
	// value's upper bound.
	hi, end := lo, len(entries)
	for hi < end {
		middle := int(uint(hi+end) >> 1)
		if lowerAfterUpper(m.compare, entries[middle].Range, value) > 0 {
			end = middle
		} else {
			hi = middle + 1
		}
	}
	return lo, hi
}

// splice replaces entries[lo:hi] with pieces, in place when the counts match.
func (m *RangeMap[T, V]) splice(lo, hi int, pieces []Entry[T, V]) {
	if len(pieces) == hi-lo {
		copy(m.entries[lo:hi], pieces)
		return
	}
	m.entries = slices.Replace(m.entries, lo, hi, pieces...)
}

// Put replaces all mappings overlapping value, preserving non-overlapping left
// and right fragments of partially covered entries. Complexity is O(log r)
// comparisons to locate the affected run plus an O(r) worst-case element shift
// (a memmove; amortized slice growth only).
func (m *RangeMap[T, V]) Put(value ranges.Range[T], mapped V) {
	if m == nil || m.compare == nil {
		panic("range_map: Put on zero-value RangeMap")
	}
	value = m.canonical(value)
	if value.IsEmpty() {
		return
	}
	lo, hi := m.connectedWindow(value)
	// The run's untouched (merely touching) entries and fragments, plus the new
	// entry, put back in lower-bound order. Only the run's two end entries can
	// survive in any form, so the replacement usually fits the fixed buffer.
	var buffer [5]Entry[T, V]
	pieces := m.subtract(buffer[:0], m.entries[lo:hi], value)
	pieces = append(pieces, Entry[T, V]{Range: value, Value: mapped})
	// Insertion sort, stable: the new entry was appended last, as before.
	for index := len(pieces) - 1; index > 0 && lowerCompare(pieces[index-1], pieces[index].Range) > 0; index-- {
		pieces[index-1], pieces[index] = pieces[index], pieces[index-1]
	}
	m.splice(lo, hi, pieces)
}

// subtract appends to result what remains of each window entry after removing
// value, in order. Entries that merely touch value are kept unchanged.
func (m *RangeMap[T, V]) subtract(result, window []Entry[T, V], value ranges.Range[T]) []Entry[T, V] {
	for _, existing := range window {
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
			// A fragment can canonicalize to empty — the remainder of an
			// unbounded entry past a finite domain's maximum is (max,max) — and
			// an empty range is never stored.
			if left {
				if fragment := m.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true); !fragment.IsEmpty() {
					result = append(result, Entry[T, V]{Range: fragment, Value: existing.Value})
				}
			}
		}
		if cutHasUpper {
			order := m.compare(cutUpper, oldUpper)
			right := !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
			if right {
				if fragment := m.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper); !fragment.IsEmpty() {
					result = append(result, Entry[T, V]{Range: fragment, Value: existing.Value})
				}
			}
		}
	}
	return result
}

// Remove deletes value's interval and preserves exact boundary fragments.
// Complexity is O(log r) comparisons to locate the affected run plus an O(r)
// worst-case element shift (a memmove; amortized slice growth only).
func (m *RangeMap[T, V]) Remove(value ranges.Range[T]) {
	if m == nil || m.compare == nil {
		panic("range_map: Remove on zero-value RangeMap")
	}
	value = m.canonical(value)
	if value.IsEmpty() {
		return
	}
	lo, hi := m.connectedWindow(value)
	if lo == hi {
		return
	}
	var buffer [4]Entry[T, V]
	m.splice(lo, hi, m.subtract(buffer[:0], m.entries[lo:hi], value))
}

// Get returns the value mapped at point. Complexity is O(log r).
func (m *RangeMap[T, V]) Get(point T) (V, bool) {
	_, value, found := m.RangeContaining(point)
	return value, found
}

// RangeContaining returns the entry containing point. Entry ranges are sorted
// and disjoint, so the only candidate is the first entry whose upper bound
// reaches point. Complexity is O(log r).
func (m *RangeMap[T, V]) RangeContaining(point T) (ranges.Range[T], V, bool) {
	if m == nil {
		return ranges.Range[T]{}, *new(V), false
	}
	entries := m.entries
	lo, hi := 0, len(entries)
	for lo < hi {
		middle := int(uint(lo+hi) >> 1)
		upper, bound, has := entries[middle].Range.UpperBound()
		order := 0
		if has {
			order = m.compare(upper, point)
		}
		if has && (order < 0 || order == 0 && bound == ranges.OpenBound) {
			lo = middle + 1
		} else {
			hi = middle
		}
	}
	if lo < len(entries) && entries[lo].Range.Contains(point) {
		return entries[lo].Range, entries[lo].Value, true
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
