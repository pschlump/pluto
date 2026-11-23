/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package range_set implements an ordered set of disjoint ranges. Mutable
// snapshots are returned by Complement and SubRangeSet, not live views.
package range_set

import (
	"cmp"
	"iter"

	"github.com/pschlump/pluto/ranges"
)

// RangeSet stores sorted, non-overlapping, non-adjacent ranges.
type RangeSet[T any] struct {
	values    []ranges.Range[T]
	compare   func(T, T) int
	domain    ranges.DiscreteDomain[T]
	hasDomain bool
}

func NewRangeSet[T cmp.Ordered]() *RangeSet[T] {
	return NewRangeSetFunc(cmp.Compare[T])
}
func NewRangeSetFunc[T any](compare func(T, T) int) *RangeSet[T] {
	if compare == nil {
		panic("range_set: nil comparison function")
	}
	return &RangeSet[T]{compare: compare}
}
func NewDiscreteRangeSet[T cmp.Ordered](domain ranges.DiscreteDomain[T]) *RangeSet[T] {
	return NewDiscreteRangeSetFunc(cmp.Compare[T], domain)
}
func NewDiscreteRangeSetFunc[T any](compare func(T, T) int, domain ranges.DiscreteDomain[T]) *RangeSet[T] {
	result := NewRangeSetFunc(compare)
	result.domain, result.hasDomain = domain, true
	return result
}

func (s *RangeSet[T]) Compare() func(T, T) int {
	if s == nil {
		return nil
	}
	return s.compare
}
func (s *RangeSet[T]) Domain() (ranges.DiscreteDomain[T], bool) {
	if s == nil || !s.hasDomain {
		return ranges.DiscreteDomain[T]{}, false
	}
	return s.domain, true
}
func (s *RangeSet[T]) canonical(value ranges.Range[T]) ranges.Range[T] {
	if s.hasDomain {
		return value.Canonical(s.domain)
	}
	return value
}
func (s *RangeSet[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}
func (s *RangeSet[T]) IsEmpty() bool { return s.Len() == 0 }
func (s *RangeSet[T]) Clear() {
	if s != nil {
		s.values = nil
	}
}

func (s *RangeSet[T]) Slice() []ranges.Range[T] {
	if s == nil {
		return nil
	}
	return append([]ranges.Range[T]{}, s.values...)
}
func (s *RangeSet[T]) All() iter.Seq[ranges.Range[T]] {
	values := s.Slice()
	return func(yield func(ranges.Range[T]) bool) {
		for _, value := range values {
			if !yield(value) {
				return
			}
		}
	}
}

func lowerCompare[T any](a, b ranges.Range[T]) int {
	aLower, aBound, aHas := a.LowerBound()
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
	order := a.Compare()(aLower, bLower)
	if order == 0 && aBound != bBound {
		if aBound == ranges.OpenBound {
			return 1
		}
		return -1
	}
	return order
}

func (s *RangeSet[T]) makeBounds(lower T, lowerOpen, hasLower bool, upper T, upperOpen, hasUpper bool) ranges.Range[T] {
	return s.canonical(makeBounds(s.compare, lower, lowerOpen, hasLower, upper, upperOpen, hasUpper))
}

func (s *RangeSet[T]) adjacent(a, b ranges.Range[T]) bool {
	if a.IsConnected(b) {
		return true
	}
	if !s.hasDomain {
		return false
	}
	nextClosed := func(left, right ranges.Range[T]) bool {
		upper, upperBound, hasUpper := left.UpperBound()
		lower, lowerBound, hasLower := right.LowerBound()
		if !hasUpper || !hasLower || upperBound == ranges.OpenBound || lowerBound == ranges.OpenBound {
			return false
		}
		next, ok := s.domain.Next(upper)
		return ok && s.compare(next, lower) == 0
	}
	return nextClosed(a, b) || nextClosed(b, a)
}

// Add inserts and coalesces connected ranges. With a discrete domain, ranges are
// canonicalized first and successor-adjacent closed ranges also coalesce.
func (s *RangeSet[T]) Add(value ranges.Range[T]) {
	if s == nil || s.compare == nil {
		panic("range_set: Add on zero-value RangeSet")
	}
	value = s.canonical(value)
	if value.IsEmpty() {
		return
	}
	merged := value
	result := make([]ranges.Range[T], 0, len(s.values)+1)
	inserted := false
	for _, existing := range s.values {
		if s.adjacent(merged, existing) {
			merged = merged.Span(existing)
			continue
		}
		if !inserted && lowerCompare(existing, merged) > 0 {
			result = append(result, merged)
			inserted = true
		}
		result = append(result, existing)
	}
	if !inserted {
		result = append(result, merged)
	}
	s.values = result
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

// Remove subtracts value, splitting existing ranges at exact boundaries.
func (s *RangeSet[T]) Remove(value ranges.Range[T]) {
	if s == nil || s.compare == nil {
		panic("range_set: Remove on zero-value RangeSet")
	}
	value = s.canonical(value)
	if value.IsEmpty() {
		return
	}
	result := make([]ranges.Range[T], 0, len(s.values))
	for _, existing := range s.values {
		if !existing.IsConnected(value) || existing.Intersection(value).IsEmpty() {
			result = append(result, existing)
			continue
		}
		if value.Encloses(existing) {
			continue
		}
		oldLower, oldLowerBound, oldHasLower := existing.LowerBound()
		oldUpper, oldUpperBound, oldHasUpper := existing.UpperBound()
		cutLower, cutLowerBound, cutHasLower := value.LowerBound()
		cutUpper, cutUpperBound, cutHasUpper := value.UpperBound()
		left := false
		if cutHasLower {
			order := s.compare(oldLower, cutLower)
			left = !oldHasLower || order < 0 || order == 0 && oldLowerBound == ranges.ClosedBound && cutLowerBound == ranges.OpenBound
		}
		if left {
			result = append(result, s.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true))
		}
		right := false
		if cutHasUpper {
			order := s.compare(cutUpper, oldUpper)
			right = !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
		}
		if right {
			result = append(result, s.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper))
		}
	}
	s.values = result
}

func (s *RangeSet[T]) Contains(value T) bool {
	_, found := s.RangeContaining(value)
	return found
}
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

// Complement returns an independent snapshot over the comparator's full universe.
func (s *RangeSet[T]) Complement() *RangeSet[T] {
	result := NewRangeSetFunc(s.compare)
	result.domain, result.hasDomain = s.domain, s.hasDomain
	if len(s.values) == 0 {
		result.values = []ranges.Range[T]{ranges.AllFunc(s.compare)}
		return result
	}
	first := s.values[0]
	if upper, bound, has := first.LowerBound(); has {
		result.values = append(result.values, s.makeBounds(*new(T), false, false, upper, bound == ranges.ClosedBound, true))
	}
	for index := 1; index < len(s.values); index++ {
		previous, next := s.values[index-1], s.values[index]
		lower, lowerBound, _ := previous.UpperBound()
		upper, upperBound, _ := next.LowerBound()
		result.values = append(result.values, s.makeBounds(lower, lowerBound == ranges.ClosedBound, true, upper, upperBound == ranges.ClosedBound, true))
	}
	last := s.values[len(s.values)-1]
	if lower, bound, has := last.UpperBound(); has {
		result.values = append(result.values, s.makeBounds(lower, bound == ranges.ClosedBound, true, *new(T), false, false))
	}
	return result
}

// SubRangeSet returns an independent intersection snapshot.
func (s *RangeSet[T]) SubRangeSet(value ranges.Range[T]) *RangeSet[T] {
	result := NewRangeSetFunc(s.compare)
	result.domain, result.hasDomain = s.domain, s.hasDomain
	for _, existing := range s.values {
		if existing.IsConnected(value) {
			intersection := existing.Intersection(value)
			if !intersection.IsEmpty() {
				result.values = append(result.values, intersection)
			}
		}
	}
	return result
}

func (s *RangeSet[T]) Clone() *RangeSet[T] {
	if s == nil {
		return nil
	}
	result := &RangeSet[T]{values: s.Slice(), compare: s.compare, domain: s.domain, hasDomain: s.hasDomain}
	return result
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
