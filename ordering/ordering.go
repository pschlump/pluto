/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package ordering provides composable comparators, stable sorted copies, and
// bounded-heap top-k algorithms.
package ordering

import (
	"cmp"
	"iter"
	"slices"
)

// Ordering compares values; negative, zero, and positive results respectively
// mean less than, equal, and greater than. Its zero value is not usable.
type Ordering[T any] struct {
	compare func(T, T) int
}

// Natural orders values with cmp.Compare.
func Natural[T cmp.Ordered]() Ordering[T] {
	return FromFunc(cmp.Compare[T])
}

// FromFunc constructs an Ordering from a comparison function.
func FromFunc[T any](compare func(T, T) int) Ordering[T] {
	if compare == nil {
		panic("ordering: nil comparison")
	}
	return Ordering[T]{compare: compare}
}

// AllEqual considers every pair of values equal.
func AllEqual[T any]() Ordering[T] {
	return FromFunc(func(T, T) int { return 0 })
}

// Compare applies the ordering to two values.
func (order Ordering[T]) Compare(left, right T) int {
	return order.compare(left, right)
}

// Reverse reverses the ordering.
func (order Ordering[T]) Reverse() Ordering[T] {
	return FromFunc(func(left, right T) int { return order.compare(right, left) })
}

// Then uses other to break ties.
func (order Ordering[T]) Then(other Ordering[T]) Ordering[T] {
	return FromFunc(func(left, right T) int {
		if result := order.compare(left, right); result != 0 {
			return result
		}
		return other.compare(left, right)
	})
}

// ByKey orders values by a derived key.
func ByKey[T, K any](key func(T) K, keyOrder Ordering[K]) Ordering[T] {
	if key == nil {
		panic("ordering: nil key function")
	}
	return FromFunc(func(left, right T) int { return keyOrder.compare(key(left), key(right)) })
}

// Lexicographical orders slices by corresponding elements, with a proper
// prefix ordered before the longer slice.
func Lexicographical[T any](element Ordering[T]) Ordering[[]T] {
	return FromFunc(func(left, right []T) int {
		limit := min(len(left), len(right))
		for index := 0; index < limit; index++ {
			if result := element.compare(left[index], right[index]); result != 0 {
				return result
			}
		}
		return cmp.Compare(len(left), len(right))
	})
}

// Min consumes values and returns the least value, if any. The first of equal
// minima is retained.
func (order Ordering[T]) Min(values iter.Seq[T]) (T, bool) {
	return order.extreme(values, false)
}

// Max consumes values and returns the greatest value, if any. The first of
// equal maxima is retained.
func (order Ordering[T]) Max(values iter.Seq[T]) (T, bool) {
	return order.extreme(values, true)
}

func (order Ordering[T]) extreme(values iter.Seq[T], maximum bool) (T, bool) {
	var result T
	found := false
	for value := range values {
		if !found {
			result, found = value, true
			continue
		}
		comparison := order.compare(value, result)
		if (!maximum && comparison < 0) || (maximum && comparison > 0) {
			result = value
		}
	}
	return result, found
}

// IsOrdered reports whether each value is no greater than its successor.
func (order Ordering[T]) IsOrdered(values iter.Seq[T]) bool {
	return order.ordered(values, false)
}

// IsStrictlyOrdered reports whether each value is less than its successor.
func (order Ordering[T]) IsStrictlyOrdered(values iter.Seq[T]) bool {
	return order.ordered(values, true)
}

func (order Ordering[T]) ordered(values iter.Seq[T], strict bool) bool {
	var previous T
	found := false
	valid := true
	for value := range values {
		if found {
			comparison := order.compare(previous, value)
			if comparison > 0 || (strict && comparison == 0) {
				valid = false
				return valid
			}
		}
		previous, found = value, true
	}
	return valid
}

// SortedCopy returns a stable sorted copy and leaves values unchanged.
func (order Ordering[T]) SortedCopy(values []T) []T {
	result := append([]T(nil), values...)
	slices.SortStableFunc(result, order.compare)
	return result
}

// LeastOf returns up to count least values in ascending order. It uses O(count)
// auxiliary storage and O(n log count) comparisons. A negative count panics.
func (order Ordering[T]) LeastOf(values iter.Seq[T], count int) []T {
	return order.top(values, count)
}

// GreatestOf returns up to count greatest values in descending order. It uses
// O(count) auxiliary storage and O(n log count) comparisons.
func (order Ordering[T]) GreatestOf(values iter.Seq[T], count int) []T {
	return order.Reverse().top(values, count)
}

func (order Ordering[T]) top(values iter.Seq[T], count int) []T {
	if count < 0 {
		panic("ordering: negative top-k count")
	}
	if count == 0 {
		return []T{}
	}
	heap := make([]T, 0, count)
	for value := range values {
		if len(heap) < count {
			heap = append(heap, value)
			order.up(heap, len(heap)-1)
			continue
		}
		if order.compare(value, heap[0]) < 0 {
			heap[0] = value
			order.down(heap)
		}
	}
	slices.SortFunc(heap, order.compare)
	return heap
}

func (order Ordering[T]) up(heap []T, index int) {
	for index > 0 {
		parent := (index - 1) / 2
		if order.compare(heap[parent], heap[index]) >= 0 {
			return
		}
		heap[parent], heap[index] = heap[index], heap[parent]
		index = parent
	}
}

func (order Ordering[T]) down(heap []T) {
	index := 0
	for {
		left := index*2 + 1
		if left >= len(heap) {
			return
		}
		worst := left
		right := left + 1
		if right < len(heap) && order.compare(heap[right], heap[left]) > 0 {
			worst = right
		}
		if order.compare(heap[index], heap[worst]) >= 0 {
			return
		}
		heap[index], heap[worst] = heap[worst], heap[index]
		index = worst
	}
}
