/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package ranges

import "unsafe"

// Signed covers built-in signed integer types.
type Signed interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Unsigned covers built-in unsigned integer types.
type Unsigned interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// DiscreteDomain supplies checked successor, predecessor, distance, and optional
// finite extrema. Distance returns false for reversed endpoints or overflow.
type DiscreteDomain[T any] struct {
	NextFunc     func(T) (T, bool)
	PrevFunc     func(T) (T, bool)
	DistanceFunc func(T, T) (uint64, bool)
	MinFunc      func() (T, bool)
	MaxFunc      func() (T, bool)
}

func (d DiscreteDomain[T]) Next(value T) (T, bool) {
	if d.NextFunc == nil {
		var zero T
		return zero, false
	}
	return d.NextFunc(value)
}
func (d DiscreteDomain[T]) Prev(value T) (T, bool) {
	if d.PrevFunc == nil {
		var zero T
		return zero, false
	}
	return d.PrevFunc(value)
}
func (d DiscreteDomain[T]) Distance(start, end T) (uint64, bool) {
	if d.DistanceFunc == nil {
		return 0, false
	}
	return d.DistanceFunc(start, end)
}
func (d DiscreteDomain[T]) Min() (T, bool) {
	if d.MinFunc == nil {
		var zero T
		return zero, false
	}
	return d.MinFunc()
}
func (d DiscreteDomain[T]) Max() (T, bool) {
	if d.MaxFunc == nil {
		var zero T
		return zero, false
	}
	return d.MaxFunc()
}

// SignedDomain returns the checked full domain of T.
func SignedDomain[T Signed]() DiscreteDomain[T] {
	var zero T
	width := uint(unsafe.Sizeof(zero)) * 8
	maximum := T((uint64(1) << (width - 1)) - 1)
	minimum := -maximum - 1
	return DiscreteDomain[T]{
		NextFunc: func(value T) (T, bool) {
			if value == maximum {
				return zero, false
			}
			return value + 1, true
		},
		PrevFunc: func(value T) (T, bool) {
			if value == minimum {
				return zero, false
			}
			return value - 1, true
		},
		DistanceFunc: func(start, end T) (uint64, bool) {
			if start > end {
				return 0, false
			}
			return uint64(int64(end)) - uint64(int64(start)), true
		},
		MinFunc: func() (T, bool) { return minimum, true },
		MaxFunc: func() (T, bool) { return maximum, true },
	}
}

// UnsignedDomain returns the checked full domain of T.
func UnsignedDomain[T Unsigned]() DiscreteDomain[T] {
	var zero T
	width := uint(unsafe.Sizeof(zero)) * 8
	maximum := ^uint64(0)
	if width < 64 {
		maximum = (uint64(1) << width) - 1
	}
	maxValue := T(maximum)
	return DiscreteDomain[T]{
		NextFunc: func(value T) (T, bool) {
			if value == maxValue {
				return zero, false
			}
			return value + 1, true
		},
		PrevFunc: func(value T) (T, bool) {
			if value == 0 {
				return zero, false
			}
			return value - 1, true
		},
		DistanceFunc: func(start, end T) (uint64, bool) {
			if start > end {
				return 0, false
			}
			return uint64(end) - uint64(start), true
		},
		MinFunc: func() (T, bool) { return zero, true },
		MaxFunc: func() (T, bool) { return maxValue, true },
	}
}

// Canonical converts a range to its discrete closed-open equivalent. Open lower
// and closed upper endpoints advance by one. An exhausted lower endpoint produces
// an empty range; an exhausted maximum upper endpoint remains closed.
func (r Range[T]) Canonical(domain DiscreteDomain[T]) Range[T] {
	if r.IsEmpty() {
		return r
	}
	result := r
	if r.hasLower && r.lowerOpen {
		value, ok := domain.Next(r.lower)
		if !ok {
			return emptyAt(r, r.lower)
		}
		result.lower, result.lowerOpen = value, false
	}
	if r.hasUpper && !r.upperOpen {
		value, ok := domain.Next(r.upper)
		if ok {
			result.upper, result.upperOpen = value, true
		}
	}
	if result.hasLower && result.hasUpper && result.compare(result.lower, result.upper) > 0 {
		return emptyAt(result, result.lower)
	}
	return result
}

func emptyAt[T any](r Range[T], value T) Range[T] {
	r.lower, r.upper = value, value
	r.hasLower, r.hasUpper = true, true
	r.lowerOpen, r.upperOpen = true, true
	return r
}
