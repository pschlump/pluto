/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package immutable

import "iter"

// Set is an immutable hash set with first-occurrence iteration order.
type Set[T any] struct {
	values []T
	index  *hashIndex[T]
	equal  func(T, T) bool
}

func NewSet[T comparable](values []T) *Set[T] {
	equal, _ := naturalFunctions[T]()
	return newSet(equal, newNaturalIndex[T](len(values)), values)
}

// NewSetFunc defensively copies values and collapses custom-equal duplicates.
func NewSetFunc[T any](equal func(T, T) bool, hash func(T) uint64, values []T) *Set[T] {
	return newSet(equal, newHashIndexSized(equal, hash, len(values)), values)
}

func newSet[T any](equal func(T, T) bool, index *hashIndex[T], values []T) *Set[T] {
	result := &Set[T]{index: index, equal: equal, values: make([]T, 0, len(values))}
	for _, value := range values {
		if index.add(value, len(result.values)) {
			result.values = append(result.values, value)
		}
	}
	if len(result.values) == 0 {
		result.values = nil
	}
	return result
}

func CopyOfSet[T any](set *Set[T]) *Set[T] { return set }

func (s *Set[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.values)
}

func (s *Set[T]) IsEmpty() bool { return s.Len() == 0 }

func (s *Set[T]) Contains(value T) bool {
	if s == nil {
		return false
	}
	_, found := s.index.get(value)
	return found
}

func (s *Set[T]) All() iter.Seq[T] {
	var values []T
	if s != nil {
		values = s.values
	}
	return sequence(values)
}

func (s *Set[T]) Slice() []T {
	if s == nil {
		return nil
	}
	return append([]T{}, s.values...)
}

// Equal reports membership equality using the receiver's equality function.
func (s *Set[T]) Equal(other *Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	if s.Len() == 0 {
		return true
	}
	for value := range s.All() {
		if !other.Contains(value) {
			return false
		}
	}
	return true
}

// SetBuilder incrementally constructs independent Set snapshots.
type SetBuilder[T any] struct {
	values  []T
	equal   func(T, T) bool
	hash    func(T) uint64
	natural func(hint int) *hashIndex[T] // set by NewSetBuilder only
}

func NewSetBuilder[T comparable]() *SetBuilder[T] {
	equal, hash := naturalFunctions[T]()
	builder := NewSetBuilderFunc(equal, hash)
	builder.natural = newNaturalIndex[T]
	return builder
}

func NewSetBuilderFunc[T any](equal func(T, T) bool, hash func(T) uint64) *SetBuilder[T] {
	if equal == nil || hash == nil {
		panic("immutable: NewSetBuilderFunc requires non-nil functions")
	}
	return &SetBuilder[T]{equal: equal, hash: hash}
}

func (b *SetBuilder[T]) Add(value T)       { b.values = append(b.values, value) }
func (b *SetBuilder[T]) AddAll(values []T) { b.values = append(b.values, values...) }

func (b *SetBuilder[T]) Build() *Set[T] {
	if b == nil {
		return nil
	}
	if b.natural != nil {
		return newSet(b.equal, b.natural(len(b.values)), b.values)
	}
	return NewSetFunc(b.equal, b.hash, b.values)
}
