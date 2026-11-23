/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package range_set_ts is the mutex-guarded twin of range_set. Iterators and
// derived sets are snapshots and opposite-way equality never holds two locks.
package range_set_ts

import (
	"cmp"
	"iter"
	"slices"
	"sync"

	"github.com/pschlump/pluto/range_set"
	"github.com/pschlump/pluto/ranges"
)

type RangeSet[T any] struct {
	lock  sync.RWMutex
	plain *range_set.RangeSet[T]
}

func NewRangeSet[T cmp.Ordered]() *RangeSet[T] {
	return &RangeSet[T]{plain: range_set.NewRangeSet[T]()}
}
func NewRangeSetFunc[T any](compare func(T, T) int) *RangeSet[T] {
	return &RangeSet[T]{plain: range_set.NewRangeSetFunc(compare)}
}
func NewDiscreteRangeSet[T cmp.Ordered](domain ranges.DiscreteDomain[T]) *RangeSet[T] {
	return &RangeSet[T]{plain: range_set.NewDiscreteRangeSet(domain)}
}
func NewDiscreteRangeSetFunc[T any](compare func(T, T) int, domain ranges.DiscreteDomain[T]) *RangeSet[T] {
	return &RangeSet[T]{plain: range_set.NewDiscreteRangeSetFunc(compare, domain)}
}
func wrap[T any](plain *range_set.RangeSet[T]) *RangeSet[T] {
	if plain == nil {
		return nil
	}
	return &RangeSet[T]{plain: plain}
}
func (s *RangeSet[T]) Add(value ranges.Range[T]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.plain.Add(value)
}
func (s *RangeSet[T]) Remove(value ranges.Range[T]) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.plain.Remove(value)
}
func (s *RangeSet[T]) Len() int {
	if s == nil || s.plain == nil {
		return 0
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.Len()
}
func (s *RangeSet[T]) IsEmpty() bool { return s.Len() == 0 }
func (s *RangeSet[T]) Clear() {
	if s == nil || s.plain == nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.plain.Clear()
}
func (s *RangeSet[T]) Compare() func(T, T) int {
	if s == nil || s.plain == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.Compare()
}
func (s *RangeSet[T]) Slice() []ranges.Range[T] {
	if s == nil || s.plain == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.Slice()
}
func (s *RangeSet[T]) All() iter.Seq[ranges.Range[T]] { return slices.Values(s.Slice()) }
func (s *RangeSet[T]) Contains(value T) bool {
	if s == nil || s.plain == nil {
		return false
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.Contains(value)
}
func (s *RangeSet[T]) RangeContaining(value T) (ranges.Range[T], bool) {
	if s == nil || s.plain == nil {
		return ranges.Range[T]{}, false
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.RangeContaining(value)
}
func (s *RangeSet[T]) Encloses(value ranges.Range[T]) bool {
	if s == nil || s.plain == nil {
		return false
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.Encloses(value)
}
func (s *RangeSet[T]) Span() (ranges.Range[T], bool) {
	if s == nil || s.plain == nil {
		return ranges.Range[T]{}, false
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.plain.Span()
}
func (s *RangeSet[T]) Complement() *RangeSet[T] {
	if s == nil || s.plain == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return wrap(s.plain.Complement())
}
func (s *RangeSet[T]) SubRangeSet(value ranges.Range[T]) *RangeSet[T] {
	if s == nil || s.plain == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return wrap(s.plain.SubRangeSet(value))
}
func (s *RangeSet[T]) Clone() *RangeSet[T] {
	if s == nil || s.plain == nil {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return wrap(s.plain.Clone())
}
func (s *RangeSet[T]) Equal(other *RangeSet[T]) bool {
	left, right := s.Slice(), other.Slice()
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !left[index].Equal(right[index]) {
			return false
		}
	}
	return true
}
