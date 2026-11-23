/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import "iter"

// List is an immutable indexed sequence. The zero value is an empty list.
type List[T any] struct {
	values []T
}

// NewList defensively copies values in slice order.
func NewList[T any](values []T) *List[T] {
	return &List[T]{values: append([]T(nil), values...)}
}

// CopyOfList returns list unchanged because it is already immutable.
func CopyOfList[T any](list *List[T]) *List[T] { return list }

func (l *List[T]) Len() int {
	if l == nil {
		return 0
	}
	return len(l.values)
}

func (l *List[T]) IsEmpty() bool { return l.Len() == 0 }

// At returns the element at index, or false when index is out of range.
func (l *List[T]) At(index int) (T, bool) {
	if l == nil || index < 0 || index >= len(l.values) {
		var zero T
		return zero, false
	}
	return l.values[index], true
}

// SubList returns the half-open range [start,end). It panics for invalid bounds.
// Large sublists are O(1) views; small slices of much larger arrays are copied
// to avoid retaining the entire backing array.
func (l *List[T]) SubList(start, end int) *List[T] {
	if l == nil || start < 0 || end < start || end > len(l.values) {
		panic("immutable: List.SubList bounds out of range")
	}
	return &List[T]{values: copySubslice(l.values[start:end])}
}

// Slice returns a fresh copy in index order.
func (l *List[T]) Slice() []T {
	if l == nil {
		return nil
	}
	return append([]T{}, l.values...)
}

func (l *List[T]) All() iter.Seq[T] {
	if l == nil {
		return sequence[T](nil)
	}
	return sequence(l.values)
}

// Reverse iterates from the last element to the first.
func (l *List[T]) Reverse() iter.Seq[T] {
	if l == nil {
		return sequence[T](nil)
	}
	return reverseSequence(l.values)
}

// Equal reports sequence equality using == on dynamic element values.
func (l *List[T]) Equal(other *List[T]) bool {
	return l.EqualFunc(other, func(a, b T) bool { return any(a) == any(b) })
}

// EqualFunc reports sequence equality using equal.
func (l *List[T]) EqualFunc(other *List[T], equal func(T, T) bool) bool {
	if l.Len() != other.Len() {
		return false
	}
	if equal == nil {
		panic("immutable: List.EqualFunc called with nil equality")
	}
	for index := 0; index < l.Len(); index++ {
		left, _ := l.At(index)
		right, _ := other.At(index)
		if !equal(left, right) {
			return false
		}
	}
	return true
}

// ListBuilder incrementally constructs independent List snapshots.
type ListBuilder[T any] struct {
	values []T
}

func NewListBuilder[T any]() *ListBuilder[T] { return &ListBuilder[T]{} }

func (b *ListBuilder[T]) Add(value T) {
	b.values = append(b.values, value)
}

func (b *ListBuilder[T]) AddAll(values []T) {
	b.values = append(b.values, values...)
}

// Build copies the builder's current contents. Later builder reuse cannot
// mutate the returned list or any earlier build.
func (b *ListBuilder[T]) Build() *List[T] {
	if b == nil {
		return NewList[T](nil)
	}
	return NewList(b.values)
}
