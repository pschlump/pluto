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
	"slices"

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

// connectedWindow returns the half-open index window [lo, hi) of the stored
// ranges connected to value. Stored ranges are sorted and disjoint, so their
// upper bounds ascend with their lower bounds and both halves of IsConnected
// are monotone over the slice: the connected ranges are one contiguous run,
// found by two binary searches. Complexity is O(log r).
func (s *RangeSet[T]) connectedWindow(value ranges.Range[T]) (int, int) {
	values := s.values
	// lo: the first stored range whose upper bound reaches value's lower bound.
	lo, end := 0, len(values)
	for lo < end {
		middle := int(uint(lo+end) >> 1)
		if lowerAfterUpper(s.compare, value, values[middle]) > 0 {
			lo = middle + 1
		} else {
			end = middle
		}
	}
	// hi: the first stored range at or after lo whose lower bound lies strictly
	// after value's upper bound.
	hi, end := lo, len(values)
	for hi < end {
		middle := int(uint(hi+end) >> 1)
		if lowerAfterUpper(s.compare, values[middle], value) > 0 {
			end = middle
		} else {
			hi = middle + 1
		}
	}
	return lo, hi
}

// Add inserts and coalesces connected ranges. With a discrete domain, ranges are
// canonicalized first and successor-adjacent closed ranges also coalesce.
// Complexity is O(log r) comparisons to locate the affected run plus an O(r)
// worst-case element shift (a memmove; amortized slice growth only).
func (s *RangeSet[T]) Add(value ranges.Range[T]) {
	if s == nil || s.compare == nil {
		panic("range_set: Add on zero-value RangeSet")
	}
	value = s.canonical(value)
	if value.IsEmpty() {
		return
	}
	lo, hi := s.connectedWindow(value)
	// Successor adjacency (discrete domains only) can reach one stored range
	// beyond each end of the connected run; stored ranges are pairwise
	// non-adjacent, so each loop takes at most one step.
	for lo > 0 && s.adjacent(value, s.values[lo-1]) {
		lo--
	}
	for hi < len(s.values) && s.adjacent(value, s.values[hi]) {
		hi++
	}
	merged := value
	for _, existing := range s.values[lo:hi] {
		merged = merged.Span(existing)
	}
	if hi-lo == 1 {
		s.values[lo] = merged
		return
	}
	s.values = slices.Replace(s.values, lo, hi, merged)
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
// Complexity is O(log r) comparisons to locate the affected run plus an O(r)
// worst-case element shift (a memmove; amortized slice growth only).
func (s *RangeSet[T]) Remove(value ranges.Range[T]) {
	if s == nil || s.compare == nil {
		panic("range_set: Remove on zero-value RangeSet")
	}
	value = s.canonical(value)
	if value.IsEmpty() {
		return
	}
	lo, hi := s.connectedWindow(value)
	if lo == hi {
		return
	}
	// Only the first and last ranges of the run can survive in any form (the
	// run's interior is enclosed by value and dropped), so the replacement
	// fits the fixed stack buffer; append would grow it otherwise.
	var buffer [4]ranges.Range[T]
	pieces := s.subtract(buffer[:0], s.values[lo:hi], value)
	if len(pieces) == hi-lo {
		copy(s.values[lo:hi], pieces)
		return
	}
	s.values = slices.Replace(s.values, lo, hi, pieces...)
}

// subtract appends to result what remains of each window range after removing
// value, in order. Window ranges that merely touch value are kept unchanged.
func (s *RangeSet[T]) subtract(result, window []ranges.Range[T], value ranges.Range[T]) []ranges.Range[T] {
	for _, existing := range window {
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
		// A fragment can canonicalize to empty — the remainder of an unbounded
		// range past a finite domain's maximum is (max,max) — and an empty
		// range is never stored.
		if left {
			if fragment := s.makeBounds(oldLower, oldLowerBound == ranges.OpenBound, oldHasLower, cutLower, cutLowerBound == ranges.ClosedBound, true); !fragment.IsEmpty() {
				result = append(result, fragment)
			}
		}
		right := false
		if cutHasUpper {
			order := s.compare(cutUpper, oldUpper)
			right = !oldHasUpper || order < 0 || order == 0 && oldUpperBound == ranges.ClosedBound && cutUpperBound == ranges.OpenBound
		}
		if right {
			if fragment := s.makeBounds(cutUpper, cutUpperBound == ranges.ClosedBound, true, oldUpper, oldUpperBound == ranges.OpenBound, oldHasUpper); !fragment.IsEmpty() {
				result = append(result, fragment)
			}
		}
	}
	return result
}

// Contains reports whether value lies in a stored range.
// Complexity is O(log r).
func (s *RangeSet[T]) Contains(value T) bool {
	_, found := s.RangeContaining(value)
	return found
}

// RangeContaining returns the stored range containing value. Stored ranges are
// sorted and disjoint, so the only candidate is the first range whose upper
// bound reaches value. Complexity is O(log r).
func (s *RangeSet[T]) RangeContaining(value T) (ranges.Range[T], bool) {
	if s == nil {
		return ranges.Range[T]{}, false
	}
	values := s.values
	lo, hi := 0, len(values)
	for lo < hi {
		middle := int(uint(lo+hi) >> 1)
		upper, bound, has := values[middle].UpperBound()
		order := 0
		if has {
			order = s.compare(upper, value)
		}
		if has && (order < 0 || order == 0 && bound == ranges.OpenBound) {
			lo = middle + 1
		} else {
			hi = middle
		}
	}
	if lo < len(values) && values[lo].Contains(value) {
		return values[lo], true
	}
	return ranges.Range[T]{}, false
}

// Encloses reports whether one stored range encloses value; an empty value is
// always enclosed. The only candidate is the first stored range whose upper
// bound reaches value's lower bound. Complexity is O(log r).
func (s *RangeSet[T]) Encloses(value ranges.Range[T]) bool {
	if value.IsEmpty() {
		return true
	}
	values := s.values
	lo, hi := 0, len(values)
	for lo < hi {
		middle := int(uint(lo+hi) >> 1)
		if lowerAfterUpper(s.compare, value, values[middle]) > 0 {
			lo = middle + 1
		} else {
			hi = middle
		}
	}
	return lo < len(values) && values[lo].Encloses(value)
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
