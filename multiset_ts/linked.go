/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset_ts

import (
	"iter"
	"sync"

	"github.com/pschlump/pluto/multiset"
)

// LinkedHashMultiset is the thread-safe insertion-ordered multiset.
type LinkedHashMultiset[T any] struct {
	lock  sync.RWMutex
	inner multiset.LinkedHashMultiset[T]
}

func NewLinkedHashMultiset[T comparable]() *LinkedHashMultiset[T] {
	return &LinkedHashMultiset[T]{inner: *multiset.NewLinkedHashMultiset[T]()}
}

func NewLinkedHashMultisetFunc[T any](eq func(T, T) bool, hash func(T) uint64) *LinkedHashMultiset[T] {
	return &LinkedHashMultiset[T]{inner: *multiset.NewLinkedHashMultisetFunc(eq, hash)}
}

func FromSliceLinked[T comparable](values []T) *LinkedHashMultiset[T] {
	result := NewLinkedHashMultiset[T]()
	result.AddSlice(values)
	return result
}

func FromSliceLinkedFunc[T any](eq func(T, T) bool, hash func(T) uint64, values []T) *LinkedHashMultiset[T] {
	result := NewLinkedHashMultisetFunc(eq, hash)
	result.AddSlice(values)
	return result
}

func (m *LinkedHashMultiset[T]) Add(value T, occurrences int) int {
	if m == nil {
		var plain *multiset.LinkedHashMultiset[T]
		return plain.Add(value, occurrences)
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Add(value, occurrences)
}
func (m *LinkedHashMultiset[T]) Remove(value T, occurrences int) int {
	if m == nil {
		return 0
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.Remove(value, occurrences)
}
func (m *LinkedHashMultiset[T]) SetCount(value T, count int) int {
	if m == nil {
		var plain *multiset.LinkedHashMultiset[T]
		return plain.SetCount(value, count)
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.inner.SetCount(value, count)
}
func (m *LinkedHashMultiset[T]) DeleteAll(value T) int { return m.SetCount(value, 0) }
func (m *LinkedHashMultiset[T]) Count(value T) int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Count(value)
}
func (m *LinkedHashMultiset[T]) Contains(value T) bool { return m.Count(value) != 0 }
func (m *LinkedHashMultiset[T]) Len() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.Len()
}
func (m *LinkedHashMultiset[T]) DistinctLen() int {
	if m == nil {
		return 0
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.DistinctLen()
}
func (m *LinkedHashMultiset[T]) IsEmpty() bool { return m.Len() == 0 }
func (m *LinkedHashMultiset[T]) Clear() {
	if m == nil {
		return
	}
	m.lock.Lock()
	m.inner.Clear()
	m.lock.Unlock()
}
func (m *LinkedHashMultiset[T]) AddSlice(values []T) {
	if len(values) == 0 {
		return
	}
	if m == nil {
		panic("multiset_ts: AddSlice called on a nil LinkedHashMultiset")
	}
	m.lock.Lock()
	m.inner.AddSlice(values)
	m.lock.Unlock()
}
func (m *LinkedHashMultiset[T]) AddIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	values := []T{}
	for value := range seq {
		values = append(values, value)
	}
	m.AddSlice(values)
}
func (m *LinkedHashMultiset[T]) RemoveSlice(values []T) {
	if m == nil {
		return
	}
	m.lock.Lock()
	m.inner.RemoveSlice(values)
	m.lock.Unlock()
}
func (m *LinkedHashMultiset[T]) RemoveIter(seq iter.Seq[T]) {
	if seq == nil {
		return
	}
	values := []T{}
	for value := range seq {
		values = append(values, value)
	}
	m.RemoveSlice(values)
}
func (m *LinkedHashMultiset[T]) Eq() func(T, T) bool {
	if m == nil {
		return nil
	}
	return m.inner.Eq()
}
func (m *LinkedHashMultiset[T]) Hash() func(T) uint64 {
	if m == nil {
		return nil
	}
	return m.inner.Hash()
}
func (m *LinkedHashMultiset[T]) EntriesSlice() []multiset.Entry[T] {
	if m == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.EntriesSlice()
}
func linkedEntries[T any](entries []multiset.Entry[T]) iter.Seq[multiset.Entry[T]] {
	return func(yield func(multiset.Entry[T]) bool) {
		for _, entry := range entries {
			if !yield(entry) {
				return
			}
		}
	}
}
func (m *LinkedHashMultiset[T]) Entries() iter.Seq[multiset.Entry[T]] {
	return linkedEntries(m.EntriesSlice())
}
func (m *LinkedHashMultiset[T]) Elements() iter.Seq[T] {
	entries := m.EntriesSlice()
	return func(yield func(T) bool) {
		for _, entry := range entries {
			if !yield(entry.Value) {
				return
			}
		}
	}
}
func (m *LinkedHashMultiset[T]) All() iter.Seq[T] {
	entries := m.EntriesSlice()
	return func(yield func(T) bool) {
		for _, entry := range entries {
			for range entry.Count {
				if !yield(entry.Value) {
					return
				}
			}
		}
	}
}
func (m *LinkedHashMultiset[T]) Slice() []T {
	values := make([]T, 0, m.Len())
	for value := range m.All() {
		values = append(values, value)
	}
	return values
}
func (m *LinkedHashMultiset[T]) HighestCountFirst() []multiset.Entry[T] {
	if m == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.inner.HighestCountFirst()
}
func (m *LinkedHashMultiset[T]) Clone() *LinkedHashMultiset[T] {
	if m == nil {
		return nil
	}
	m.lock.RLock()
	defer m.lock.RUnlock()
	return &LinkedHashMultiset[T]{inner: *m.inner.Clone()}
}
func cloneLinked[T any](m *LinkedHashMultiset[T]) *LinkedHashMultiset[T] {
	if m == nil {
		return &LinkedHashMultiset[T]{}
	}
	return m.Clone()
}

func (m *LinkedHashMultiset[T]) ContainsOccurrences(other *LinkedHashMultiset[T]) bool {
	if other == nil {
		return true
	}
	left, right := cloneLinked(m), cloneLinked(other)
	return left.inner.ContainsOccurrences(&right.inner)
}
func (m *LinkedHashMultiset[T]) Equal(other *LinkedHashMultiset[T]) bool {
	left, right := cloneLinked(m), cloneLinked(other)
	return left.inner.Equal(&right.inner)
}
func (m *LinkedHashMultiset[T]) Union(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := cloneLinked(left), cloneLinked(right)
	m.lock.Lock()
	m.inner.Union(&a.inner, &b.inner)
	m.lock.Unlock()
}
func (m *LinkedHashMultiset[T]) Sum(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := cloneLinked(left), cloneLinked(right)
	m.lock.Lock()
	m.inner.Sum(&a.inner, &b.inner)
	m.lock.Unlock()
}
func (m *LinkedHashMultiset[T]) Intersection(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := cloneLinked(left), cloneLinked(right)
	m.lock.Lock()
	m.inner.Intersection(&a.inner, &b.inner)
	m.lock.Unlock()
}
func (m *LinkedHashMultiset[T]) Minus(left, right *LinkedHashMultiset[T]) {
	if m == nil {
		return
	}
	a, b := cloneLinked(left), cloneLinked(right)
	m.lock.Lock()
	m.inner.Minus(&a.inner, &b.inner)
	m.lock.Unlock()
}
