/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package ranges implements immutable open/closed/unbounded ranges, checked
// discrete domains, and lazy contiguous sets.
package ranges

import "cmp"

// BoundType identifies whether an endpoint is open or closed.
type BoundType uint8

const (
	OpenBound BoundType = iota
	ClosedBound
)

// Range is an immutable interval over a comparator-ordered type. The zero value
// is invalid; use a constructor.
type Range[T any] struct {
	lower, upper         T
	hasLower, hasUpper   bool
	lowerOpen, upperOpen bool
	compare              func(T, T) int
}

func makeRange[T any](compare func(T, T) int, lower T, lowerOpen bool, hasLower bool, upper T, upperOpen bool, hasUpper bool, validate bool) Range[T] {
	if compare == nil {
		panic("ranges: nil comparison function")
	}
	if validate && hasLower && hasUpper {
		order := compare(lower, upper)
		if order > 0 || order == 0 && lowerOpen && upperOpen {
			panic("ranges: invalid or reversed endpoints")
		}
	}
	return Range[T]{
		lower: lower, upper: upper, hasLower: hasLower, hasUpper: hasUpper,
		lowerOpen: lowerOpen, upperOpen: upperOpen, compare: compare,
	}
}

func Open[T cmp.Ordered](lower, upper T) Range[T] {
	return OpenFunc(cmp.Compare[T], lower, upper)
}
func OpenFunc[T any](compare func(T, T) int, lower, upper T) Range[T] {
	return makeRange(compare, lower, true, true, upper, true, true, true)
}
func Closed[T cmp.Ordered](lower, upper T) Range[T] {
	return ClosedFunc(cmp.Compare[T], lower, upper)
}
func ClosedFunc[T any](compare func(T, T) int, lower, upper T) Range[T] {
	return makeRange(compare, lower, false, true, upper, false, true, true)
}
func ClosedOpen[T cmp.Ordered](lower, upper T) Range[T] {
	return ClosedOpenFunc(cmp.Compare[T], lower, upper)
}
func ClosedOpenFunc[T any](compare func(T, T) int, lower, upper T) Range[T] {
	return makeRange(compare, lower, false, true, upper, true, true, true)
}
func OpenClosed[T cmp.Ordered](lower, upper T) Range[T] {
	return OpenClosedFunc(cmp.Compare[T], lower, upper)
}
func OpenClosedFunc[T any](compare func(T, T) int, lower, upper T) Range[T] {
	return makeRange(compare, lower, true, true, upper, false, true, true)
}
func GreaterThan[T cmp.Ordered](value T) Range[T] {
	return GreaterThanFunc(cmp.Compare[T], value)
}
func GreaterThanFunc[T any](compare func(T, T) int, value T) Range[T] {
	return makeRange(compare, value, true, true, *new(T), false, false, true)
}
func AtLeast[T cmp.Ordered](value T) Range[T] { return AtLeastFunc(cmp.Compare[T], value) }
func AtLeastFunc[T any](compare func(T, T) int, value T) Range[T] {
	return makeRange(compare, value, false, true, *new(T), false, false, true)
}
func LessThan[T cmp.Ordered](value T) Range[T] { return LessThanFunc(cmp.Compare[T], value) }
func LessThanFunc[T any](compare func(T, T) int, value T) Range[T] {
	return makeRange(compare, *new(T), false, false, value, true, true, true)
}
func AtMost[T cmp.Ordered](value T) Range[T] { return AtMostFunc(cmp.Compare[T], value) }
func AtMostFunc[T any](compare func(T, T) int, value T) Range[T] {
	return makeRange(compare, *new(T), false, false, value, false, true, true)
}
func All[T cmp.Ordered]() Range[T] { return AllFunc(cmp.Compare[T]) }
func AllFunc[T any](compare func(T, T) int) Range[T] {
	return makeRange(compare, *new(T), false, false, *new(T), false, false, true)
}

// Compare returns the range's comparator, or nil for a zero value.
func (r Range[T]) Compare() func(T, T) int { return r.compare }

func (r Range[T]) HasLowerBound() bool { return r.hasLower }
func (r Range[T]) HasUpperBound() bool { return r.hasUpper }
func (r Range[T]) LowerBound() (T, BoundType, bool) {
	bound := ClosedBound
	if r.lowerOpen {
		bound = OpenBound
	}
	return r.lower, bound, r.hasLower
}
func (r Range[T]) UpperBound() (T, BoundType, bool) {
	bound := ClosedBound
	if r.upperOpen {
		bound = OpenBound
	}
	return r.upper, bound, r.hasUpper
}
func (r Range[T]) LowerEndpoint() (T, bool) { return r.lower, r.hasLower }
func (r Range[T]) UpperEndpoint() (T, bool) { return r.upper, r.hasUpper }

func (r Range[T]) Contains(value T) bool {
	if r.compare == nil {
		return false
	}
	if r.hasLower {
		order := r.compare(value, r.lower)
		if order < 0 || order == 0 && r.lowerOpen {
			return false
		}
	}
	if r.hasUpper {
		order := r.compare(value, r.upper)
		if order > 0 || order == 0 && r.upperOpen {
			return false
		}
	}
	return true
}

func (r Range[T]) ContainsAll(values []T) bool {
	for _, value := range values {
		if !r.Contains(value) {
			return false
		}
	}
	return true
}

// IsEmpty reports whether the range contains no points. Internal intersection
// and canonicalization can produce an empty (a,a), although its public
// constructor rejects that form.
func (r Range[T]) IsEmpty() bool {
	return r.hasLower && r.hasUpper && r.compare(r.lower, r.upper) == 0 && (r.lowerOpen || r.upperOpen)
}

// Encloses reports whether every point in other belongs to r.
func (r Range[T]) Encloses(other Range[T]) bool {
	return r.compareLower(other) <= 0 && r.compareUpper(other) >= 0
}

func (r Range[T]) compareLower(other Range[T]) int {
	if !r.hasLower {
		if other.hasLower {
			return -1
		}
		return 0
	}
	if !other.hasLower {
		return 1
	}
	order := r.compare(r.lower, other.lower)
	if order == 0 && r.lowerOpen != other.lowerOpen {
		if r.lowerOpen {
			return 1
		}
		return -1
	}
	return order
}

func (r Range[T]) compareUpper(other Range[T]) int {
	if !r.hasUpper {
		if other.hasUpper {
			return 1
		}
		return 0
	}
	if !other.hasUpper {
		return -1
	}
	order := r.compare(r.upper, other.upper)
	if order == 0 && r.upperOpen != other.upperOpen {
		if r.upperOpen {
			return -1
		}
		return 1
	}
	return order
}

// IsConnected reports whether ranges overlap or touch at compatible endpoints.
func (r Range[T]) IsConnected(other Range[T]) bool {
	return compareLowerUpper(r, other) <= 0 && compareLowerUpper(other, r) <= 0
}

func compareLowerUpper[T any](left, right Range[T]) int {
	if !left.hasLower || !right.hasUpper {
		return -1
	}
	order := left.compare(left.lower, right.upper)
	if order == 0 && left.lowerOpen && right.upperOpen {
		return 1
	}
	return order
}

// Intersection returns the shared range and panics when the inputs are not
// connected. Touching open endpoints can produce an internal empty (a,a).
func (r Range[T]) Intersection(other Range[T]) Range[T] {
	if !r.IsConnected(other) {
		panic("ranges: intersection of disconnected ranges")
	}
	result := r
	if r.compareLower(other) < 0 {
		result.lower, result.hasLower, result.lowerOpen = other.lower, other.hasLower, other.lowerOpen
	}
	if r.compareUpper(other) > 0 {
		result.upper, result.hasUpper, result.upperOpen = other.upper, other.hasUpper, other.upperOpen
	}
	return result
}

// Span returns the smallest range enclosing both inputs, including any gap.
func (r Range[T]) Span(other Range[T]) Range[T] {
	result := r
	if r.compareLower(other) > 0 {
		result.lower, result.hasLower, result.lowerOpen = other.lower, other.hasLower, other.lowerOpen
	} else if r.hasLower && other.hasLower && r.compare(r.lower, other.lower) == 0 {
		result.lowerOpen = r.lowerOpen && other.lowerOpen
	}
	if r.compareUpper(other) < 0 {
		result.upper, result.hasUpper, result.upperOpen = other.upper, other.hasUpper, other.upperOpen
	} else if r.hasUpper && other.hasUpper && r.compare(r.upper, other.upper) == 0 {
		result.upperOpen = r.upperOpen && other.upperOpen
	}
	return result
}

// Equal reports identical bounds under the receiver's comparator.
func (r Range[T]) Equal(other Range[T]) bool {
	if r.hasLower != other.hasLower || r.hasUpper != other.hasUpper {
		return false
	}
	if r.hasLower && (r.lowerOpen != other.lowerOpen || r.compare(r.lower, other.lower) != 0) {
		return false
	}
	return !r.hasUpper || r.upperOpen == other.upperOpen && r.compare(r.upper, other.upper) == 0
}
