/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import (
	"cmp"
	"iter"
	"slices"
)

// SortedSet is an immutable comparator-ordered set with random access.
type SortedSet[T any] struct {
	values  []T
	compare func(T, T) int
}

func NewSortedSet[T cmp.Ordered](values []T) *SortedSet[T] {
	return NewSortedSetFunc(cmp.Compare[T], values)
}

func NewSortedSetFunc[T any](compare func(T, T) int, values []T) *SortedSet[T] {
	if compare == nil {
		panic("immutable: NewSortedSetFunc called with nil comparison")
	}
	ordered := append([]T(nil), values...)
	slices.SortStableFunc(ordered, compare)
	unique := 0
	for _, value := range ordered {
		if unique == 0 || compare(ordered[unique-1], value) != 0 {
			ordered[unique] = value
			unique++
		}
	}
	var zero T
	for index := unique; index < len(ordered); index++ {
		ordered[index] = zero
	}
	return &SortedSet[T]{values: slices.Clip(ordered[:unique]), compare: compare}
}

func CopyOfSortedSet[T any](set *SortedSet[T]) *SortedSet[T] { return set }

func (s *SortedSet[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}

func (s *SortedSet[T]) IsEmpty() bool { return s.Len() == 0 }

func (s *SortedSet[T]) At(index int) (T, bool) {
	if s == nil || index < 0 || index >= len(s.values) {
		var zero T
		return zero, false
	}
	return s.values[index], true
}

// Rank returns the number of elements strictly less than value.
func (s *SortedSet[T]) Rank(value T) int {
	if s == nil || s.compare == nil {
		return 0
	}
	return lowerBound(s.values, value, s.compare)
}

func (s *SortedSet[T]) Contains(value T) bool {
	if s == nil || s.compare == nil {
		return false
	}
	position := s.Rank(value)
	return position < len(s.values) && s.compare(s.values[position], value) == 0
}

func (s *SortedSet[T]) Min() (T, bool) { return s.At(0) }
func (s *SortedSet[T]) Max() (T, bool) { return s.At(s.Len() - 1) }

func (s *SortedSet[T]) navigation(value T, offset int, allowEqual bool) (T, bool) {
	if s == nil || s.compare == nil {
		var zero T
		return zero, false
	}
	position := lowerBound(s.values, value, s.compare)
	if offset < 0 {
		if position < len(s.values) && allowEqual && s.compare(s.values[position], value) == 0 {
			// Floor uses the equal position below.
		} else {
			position--
		}
	} else if !allowEqual && position < len(s.values) && s.compare(s.values[position], value) == 0 {
		position++
	}
	return s.At(position)
}

func (s *SortedSet[T]) Lower(value T) (T, bool)  { return s.navigation(value, -1, false) }
func (s *SortedSet[T]) Floor(value T) (T, bool)  { return s.navigation(value, -1, true) }
func (s *SortedSet[T]) Ceil(value T) (T, bool)   { return s.navigation(value, 1, true) }
func (s *SortedSet[T]) Higher(value T) (T, bool) { return s.navigation(value, 1, false) }

// SubRange returns values between the specified bounds. Equal endpoints produce
// an empty set when either bound excludes the endpoint.
func (s *SortedSet[T]) SubRange(from T, fromInclusive bool, to T, toInclusive bool) *SortedSet[T] {
	if s == nil || s.compare == nil || s.compare(from, to) > 0 {
		panic("immutable: invalid SortedSet subrange")
	}
	start := lowerBound(s.values, from, s.compare)
	if !fromInclusive && start < len(s.values) && s.compare(s.values[start], from) == 0 {
		start++
	}
	end := lowerBound(s.values, to, s.compare)
	if toInclusive && end < len(s.values) && s.compare(s.values[end], to) == 0 {
		end++
	}
	if start > end {
		start = end
	}
	return &SortedSet[T]{values: copySubslice(s.values[start:end]), compare: s.compare}
}

func (s *SortedSet[T]) Head(to T, inclusive bool) *SortedSet[T] {
	if s == nil {
		return &SortedSet[T]{}
	}
	end := lowerBound(s.values, to, s.compare)
	if inclusive && end < len(s.values) && s.compare(s.values[end], to) == 0 {
		end++
	}
	return &SortedSet[T]{values: copySubslice(s.values[:end]), compare: s.compare}
}

func (s *SortedSet[T]) Tail(from T, inclusive bool) *SortedSet[T] {
	if s == nil {
		return &SortedSet[T]{}
	}
	start := lowerBound(s.values, from, s.compare)
	if !inclusive && start < len(s.values) && s.compare(s.values[start], from) == 0 {
		start++
	}
	return &SortedSet[T]{values: copySubslice(s.values[start:]), compare: s.compare}
}

func (s *SortedSet[T]) Slice() []T {
	if s == nil {
		return nil
	}
	return append([]T{}, s.values...)
}

func (s *SortedSet[T]) All() iter.Seq[T] {
	if s == nil {
		return sequence[T](nil)
	}
	return sequence(s.values)
}

func (s *SortedSet[T]) Reverse() iter.Seq[T] {
	if s == nil {
		return sequence[T](nil)
	}
	return reverseSequence(s.values)
}

func (s *SortedSet[T]) Equal(other *SortedSet[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	if s.Len() == 0 {
		return true
	}
	for index, value := range s.values {
		if s.compare(value, other.values[index]) != 0 {
			return false
		}
	}
	return true
}

// SortedSetBuilder incrementally constructs independent SortedSet snapshots.
type SortedSetBuilder[T any] struct {
	values  []T
	compare func(T, T) int
}

func NewSortedSetBuilder[T cmp.Ordered]() *SortedSetBuilder[T] {
	return NewSortedSetBuilderFunc(cmp.Compare[T])
}

func NewSortedSetBuilderFunc[T any](compare func(T, T) int) *SortedSetBuilder[T] {
	if compare == nil {
		panic("immutable: NewSortedSetBuilderFunc called with nil comparison")
	}
	return &SortedSetBuilder[T]{compare: compare}
}

func (b *SortedSetBuilder[T]) Add(value T)       { b.values = append(b.values, value) }
func (b *SortedSetBuilder[T]) AddAll(values []T) { b.values = append(b.values, values...) }

func (b *SortedSetBuilder[T]) Build() *SortedSet[T] {
	if b == nil {
		return nil
	}
	return NewSortedSetFunc(b.compare, b.values)
}

func lowerBound[S any, P any](values []S, value P, compare func(S, P) int) int {
	low, high := 0, len(values)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if compare(values[middle], value) < 0 {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}
